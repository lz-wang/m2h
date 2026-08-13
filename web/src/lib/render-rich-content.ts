// Rich-content enhancement for rendered Markdown HTML.
//
// m2h keeps its Markdown parser on stable GFM/CommonMark semantics; math and
// diagram support is an HTML presentation layer applied after the browser
// receives the body. This module is the single entry point so callers never
// have to know about KaTeX or Mermaid individually.
//
// Mermaid runs before KaTeX so KaTeX never scans raw diagram source code.
// Both runtimes are the shared /runtime/* assets the preview server embeds
// (the same copy convert writes into .m2h/), loaded through the runtime loader
// only when the document contains diagrams or math.

import type { ResolvedMode } from "../model";
import {
  loadKatex,
  loadMermaid,
  type MathAutoRenderDelimiter,
  type MermaidRuntime,
} from "./runtime-loader";

const COPY_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><rect x="5.25" y="5.25" width="7.25" height="7.25" rx="1.25"></rect><path d="M10.75 5.25V3.5c0-.69-.56-1.25-1.25-1.25H3.5c-.69 0-1.25.56-1.25 1.25v6c0 .69.56 1.25 1.25 1.25h1.75"></path></svg>';
const COPIED_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m3.5 8.25 2.1 2.1 4.9-4.9"></path></svg>';
const COPY_FAILED_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m4.5 4.5 7 7m0-7-7 7"></path></svg>';

// "$$" must precede "$" so the inline delimiter does not swallow the block
// delimiter first.
const MATH_DELIMITERS: MathAutoRenderDelimiter[] = [
  { left: "$$", right: "$$", display: true },
  { left: "\\[", right: "\\]", display: true },
  { left: "\\(", right: "\\)", display: false },
  { left: "$", right: "$", display: false },
];

/**
 * Enhance already-rendered Markdown HTML inside `root` by rendering Mermaid
 * diagrams and KaTeX math. Safe to call repeatedly; errors from individual
 * blocks are suppressed so a broken diagram never breaks the whole document.
 *
 * Each runtime is loaded only when the document actually uses it — Mermaid
 * requires a fenced `language-mermaid` block, KaTeX a math delimiter — so a
 * plain document preview never downloads the multi-megabyte diagram runtime.
 *
 * `mode` is the resolved light/dark theme. Mermaid bakes diagram colors in at
 * render time, so it is configured with the matching official theme (`default`
 * for light, `dark` for dark) on each call; switching theme therefore requires
 * re-running this function so diagrams regenerate in the new palette.
 *
 * `isCurrent` is an optional freshness check consulted after Mermaid resolves.
 * Because Mermaid renders asynchronously, a slow diagram can finish after the
 * caller has swapped `root` for a different document; passing `isCurrent`
 * keeps such a stale render from applying KaTeX to content that no longer
 * belongs to it.
 */
export async function renderRichContent(
  root: HTMLElement,
  mode: ResolvedMode,
  isCurrent?: () => boolean,
): Promise<void> {
  addCodeCopyButtons(root);
  if (hasMermaidBlocks(root)) {
    const mermaid = await loadMermaid();
    ensureMermaidInitialized(mode, mermaid);
    await renderMermaid(mermaid, root, isCurrent);
  }
  if (isCurrent !== undefined && !isCurrent()) {
    return;
  }
  if (hasMathText(root)) {
    const renderMathInElement = await loadKatex();
    renderMathInElement(root, {
      delimiters: MATH_DELIMITERS,
      throwOnError: false,
    });
  }
}

function hasMermaidBlocks(root: HTMLElement): boolean {
  return root.querySelector("pre > code.language-mermaid") !== null;
}

// Matches the delimiters handed to KaTeX: every math span contains "$", "\("
// or "\[" in its text content. The check is deliberately conservative — a
// false positive only costs one runtime scan, a false negative is impossible.
function hasMathText(root: HTMLElement): boolean {
  const text = root.textContent;
  return (
    text !== null &&
    (text.includes("$") || text.includes("\\(") || text.includes("\\["))
  );
}

function addCodeCopyButtons(root: HTMLElement): void {
  for (const pre of root.querySelectorAll<HTMLPreElement>("pre")) {
    const code = pre.firstElementChild;
    if (!(code instanceof HTMLElement) || code.tagName !== "CODE") {
      continue;
    }
    if (pre.querySelector(".m2h-code-copy") !== null) {
      continue;
    }

    const button = document.createElement("button");
    button.type = "button";
    button.className = "m2h-code-copy";
    button.innerHTML = COPY_ICON;
    button.setAttribute("aria-label", "复制代码");
    button.title = "复制代码";
    button.addEventListener("click", () => {
      void copyCode(code.textContent ?? "").then((copied) => {
        setCopyStatus(button, copied);
      });
    });
    pre.append(button);
  }
}

async function copyCode(value: string): Promise<boolean> {
  // navigator.clipboard is deliberately only attempted in a secure context.
  // m2h serves previews over HTTP by default, where execCommand remains the
  // browser-compatible, user-gesture fallback.
  if (window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch {
      // Fall through when clipboard permission is unavailable or denied.
    }
  }

  const textarea = document.createElement("textarea");
  textarea.value = value;
  textarea.setAttribute("aria-hidden", "true");
  textarea.style.cssText = "position:fixed;left:-9999px;top:0;opacity:0";
  document.body.append(textarea);
  textarea.select();
  try {
    return document.execCommand("copy");
  } catch {
    return false;
  } finally {
    textarea.remove();
  }
}

function setCopyStatus(button: HTMLButtonElement, copied: boolean): void {
  button.innerHTML = copied ? COPIED_ICON : COPY_FAILED_ICON;
  button.dataset.copyState = copied ? "success" : "error";
  button.setAttribute(
    "aria-label",
    copied ? "代码已复制" : "复制代码失败，请手动复制",
  );
  button.title = copied ? "已复制" : "复制失败，请手动复制";

  window.setTimeout(() => {
    button.innerHTML = COPY_ICON;
    button.removeAttribute("data-copy-state");
    button.setAttribute("aria-label", "复制代码");
    button.title = "复制代码";
  }, 2_000);
}

// Mermaid's official light theme is "default" and dark theme is "dark". The
// module tracks which one is active so `initialize` only runs when the resolved
// theme actually changes — never on every render — while still reconfiguring
// promptly when the user toggles between light and dark.
type MermaidTheme = "default" | "dark";

let currentMermaidTheme: MermaidTheme | null = null;

// Each rendered diagram keeps its source text here rather than in a data
// attribute: Mermaid source can be long, and the WeakMap avoids leaking it once
// the container leaves the DOM. Retaining the source lets a later theme switch
// regenerate the SVG without re-parsing the document body.
const mermaidSources = new WeakMap<HTMLElement, string>();

// Monotonic id handed to every mermaid.render call so each offscreen render
// gets a unique container id (Mermaid keys its internal cache by this id).
let mermaidRenderSequence = 0;

function ensureMermaidInitialized(
  mode: ResolvedMode,
  mermaid: MermaidRuntime,
): void {
  const theme: MermaidTheme = mode === "dark" ? "dark" : "default";
  if (currentMermaidTheme === theme) {
    return;
  }
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    theme,
  });
  currentMermaidTheme = theme;
}

// Render one diagram offscreen via mermaid.render and swap the resulting SVG in
// atomically, so the container never flashes back to source text while the new
// palette resolves. Returns false when the render is no longer current, telling
// the caller to abort the remaining targets; a render that throws leaves any
// existing SVG in place so one broken diagram never breaks the document.
async function paintMermaidTarget(
  mermaid: MermaidRuntime,
  target: HTMLElement,
  source: string,
  isCurrent?: () => boolean,
): Promise<boolean> {
  try {
    const result = await mermaid.render(
      `m2h-mermaid-${++mermaidRenderSequence}`,
      source,
    );
    if (isCurrent !== undefined && !isCurrent()) {
      return false;
    }
    target.innerHTML = result.svg;
    result.bindFunctions?.(target);
  } catch {
    // Leave the existing content in place; a single bad diagram is isolated.
  }
  return true;
}

async function renderMermaid(
  mermaid: MermaidRuntime,
  root: HTMLElement,
  isCurrent?: () => boolean,
): Promise<void> {
  const targets: HTMLElement[] = [];
  for (const code of root.querySelectorAll<HTMLElement>(
    "pre > code.language-mermaid",
  )) {
    const pre = code.parentElement;
    if (!(pre instanceof HTMLPreElement)) {
      continue;
    }
    const source = code.textContent ?? "";
    const container = document.createElement("div");
    container.className = "mermaid";
    container.textContent = source;
    mermaidSources.set(container, source);
    pre.replaceWith(container);
    targets.push(container);
  }

  for (const target of targets) {
    const source = mermaidSources.get(target);
    if (source === undefined) {
      continue;
    }
    if (!(await paintMermaidTarget(mermaid, target, source, isCurrent))) {
      return;
    }
  }
}

// Re-render only existing Mermaid diagrams in a new theme, leaving the rest of
// the document body untouched. Used on a light/dark switch: because Mermaid
// bakes colors into the SVG at render time, only the diagrams need regenerating,
// while paragraphs, KaTeX, and copy controls keep both their DOM identity and
// focus.
export async function rerenderMermaid(
  root: HTMLElement,
  mode: ResolvedMode,
  isCurrent?: () => boolean,
): Promise<void> {
  const targets = Array.from(
    root.querySelectorAll<HTMLElement>(".mermaid"),
  ).filter((target) => mermaidSources.has(target));

  if (targets.length === 0) {
    return;
  }

  const mermaid = await loadMermaid();
  ensureMermaidInitialized(mode, mermaid);

  for (const target of targets) {
    const source = mermaidSources.get(target);
    if (source === undefined) {
      continue;
    }
    if (!(await paintMermaidTarget(mermaid, target, source, isCurrent))) {
      return;
    }
  }
}
