// Rich-content enhancement for rendered Markdown HTML.

import type { ResolvedMode } from "../model";
import {
  loadKatex,
  loadMermaid,
  type MathAutoRenderDelimiter,
  type MermaidRuntime,
} from "./runtime-loader";

const HEADING_SELECTOR = "h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]";
const HEADING_LINK_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>';
const COPY_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><rect x="5.25" y="5.25" width="7.25" height="7.25" rx="1.25"></rect><path d="M10.75 5.25V3.5c0-.69-.56-1.25-1.25-1.25H3.5c-.69 0-1.25.56-1.25 1.25v6c0 .69.56 1.25 1.25 1.25h1.75"></path></svg>';
const COPIED_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m3.5 8.25 2.1 2.1 4.9-4.9"></path></svg>';
const COPY_FAILED_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m4.5 4.5 7 7m0-7-7 7"></path></svg>';

const MATH_DELIMITERS: MathAutoRenderDelimiter[] = [
  { left: "$$", right: "$$", display: true },
  { left: "\\[", right: "\\]", display: true },
  { left: "\\(", right: "\\)", display: false },
  { left: "$", right: "$", display: false },
];

export async function renderRichContent(
  root: HTMLElement,
  mode: ResolvedMode,
  isCurrent?: () => boolean,
): Promise<void> {
  addHeadingAnchors(root);
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
    if (isCurrent !== undefined && !isCurrent()) {
      return;
    }
    renderMathInElement(root, {
      delimiters: MATH_DELIMITERS,
      throwOnError: false,
    });
  }
  if (isCurrent !== undefined && !isCurrent()) {
    return;
  }

  // Mermaid and KaTeX can change the height above a deep-linked heading.
  // Restore the fragment after all rich-content layout work has completed.
  restoreCurrentHash(root);
}

function hasMermaidBlocks(root: HTMLElement): boolean {
  return root.querySelector("pre > code.language-mermaid") !== null;
}

function hasMathText(root: HTMLElement): boolean {
  const text = root.textContent;
  return (
    text !== null &&
    (text.includes("$") || text.includes("\\(") || text.includes("\\["))
  );
}

function addHeadingAnchors(root: HTMLElement): void {
  for (const heading of root.querySelectorAll<HTMLElement>(HEADING_SELECTOR)) {
    if (
      heading.id === "" ||
      heading.querySelector(".m2h-heading-anchor") !== null
    ) {
      continue;
    }

    const headingText = heading.textContent?.trim() ?? "";
    const anchor = document.createElement("a");
    anchor.className = "m2h-heading-anchor";
    anchor.href = `#${encodeURIComponent(heading.id)}`;
    anchor.innerHTML = HEADING_LINK_ICON;
    anchor.setAttribute(
      "aria-label",
      headingText === "" ? "链接到此标题" : `链接到标题「${headingText}」`,
    );
    anchor.title = "链接到此标题";
    anchor.addEventListener("click", (event) => {
      // Modified clicks stay native so the permalink can open in a new tab.
      // Stop bubbling so the preview router does not reload the same document.
      event.stopPropagation();
      if (
        event.button !== 0 ||
        event.altKey ||
        event.ctrlKey ||
        event.metaKey ||
        event.shiftKey
      ) {
        return;
      }

      event.preventDefault();
      const reduceMotion = window.matchMedia(
        "(prefers-reduced-motion: reduce)",
      ).matches;
      heading.scrollIntoView({
        block: "start",
        behavior: reduceMotion ? "auto" : "smooth",
      });
      replaceLocationHash(heading.id);
    });
    heading.prepend(anchor);
  }
}

function restoreCurrentHash(root: HTMLElement): void {
  const id = readLocationHashID();
  if (id === null) {
    return;
  }
  const target = document.getElementById(id);
  if (target !== null && root.contains(target)) {
    target.scrollIntoView({ block: "start" });
  }
}

function replaceLocationHash(id: string): void {
  if (readLocationHashID() === id) {
    return;
  }
  window.history.replaceState(
    window.history.state,
    "",
    `#${encodeURIComponent(id)}`,
  );
}

function readLocationHashID(): string | null {
  const encoded = window.location.hash.slice(1);
  if (encoded === "") {
    return null;
  }
  try {
    return decodeURIComponent(encoded);
  } catch {
    return encoded;
  }
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
  if (window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(value);
      return true;
    } catch {
      // Fall through to the browser-compatible user-gesture fallback.
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

type MermaidTheme = "default" | "dark";

let currentMermaidTheme: MermaidTheme | null = null;
const mermaidSources = new WeakMap<HTMLElement, string>();
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
    // Keep existing content when one diagram fails.
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
