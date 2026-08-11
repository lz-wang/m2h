// Rich-content enhancement for rendered Markdown HTML.
//
// m2h keeps its Markdown parser on stable GFM/CommonMark semantics; math and
// diagram support is an HTML presentation layer applied after the browser
// receives the body. This module is the single entry point so callers never
// have to know about KaTeX or Mermaid individually.
//
// Mermaid runs before KaTeX so KaTeX never scans raw diagram source code.

import renderMathInElement from "katex/contrib/auto-render";
// KaTeX's stylesheet is part of the runtime presentation; importing it here lets
// Vite bundle the CSS (and the font references it depends on) with the WebUI.
import "katex/dist/katex.min.css";
import mermaid from "mermaid";

let mermaidInitialized = false;

function ensureMermaidInitialized(): void {
  if (mermaidInitialized) {
    return;
  }
  mermaid.initialize({
    startOnLoad: false,
    securityLevel: "strict",
    // "neutral" keeps diagrams legible in both light and dark without
    // re-rendering when the theme is toggled at runtime.
    theme: "neutral",
  });
  mermaidInitialized = true;
}

/**
 * Enhance already-rendered Markdown HTML inside `root` by rendering Mermaid
 * diagrams and KaTeX math. Safe to call repeatedly; errors from individual
 * blocks are suppressed so a broken diagram never breaks the whole document.
 *
 * `isCurrent` is an optional freshness check consulted after Mermaid resolves.
 * Because Mermaid renders asynchronously, a slow diagram can finish after the
 * caller has swapped `root` for a different document; passing `isCurrent`
 * keeps such a stale render from applying KaTeX to content that no longer
 * belongs to it.
 */
export async function renderRichContent(
  root: HTMLElement,
  isCurrent?: () => boolean,
): Promise<void> {
  ensureMermaidInitialized();
  await renderMermaid(root);
  if (isCurrent !== undefined && !isCurrent()) {
    return;
  }
  renderMath(root);
}

function renderMath(root: HTMLElement): void {
  renderMathInElement(root, {
    // "$$" must precede "$" so the inline delimiter does not swallow the
    // block delimiter first.
    delimiters: [
      { left: "$$", right: "$$", display: true },
      { left: "\\[", right: "\\]", display: true },
      { left: "\\(", right: "\\)", display: false },
      { left: "$", right: "$", display: false },
    ],
    throwOnError: false,
  });
}

async function renderMermaid(root: HTMLElement): Promise<void> {
  const targets: HTMLElement[] = [];
  for (const code of root.querySelectorAll<HTMLElement>(
    "pre > code.language-mermaid",
  )) {
    const pre = code.parentElement;
    if (!(pre instanceof HTMLPreElement)) {
      continue;
    }
    const container = document.createElement("div");
    container.className = "mermaid";
    container.textContent = code.textContent ?? "";
    pre.replaceWith(container);
    targets.push(container);
  }

  if (targets.length === 0) {
    return;
  }

  await mermaid.run({
    nodes: targets,
    suppressErrors: true,
  });
}
