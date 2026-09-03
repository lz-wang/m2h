// Rich-content enhancement for rendered Markdown HTML.
//
// m2h keeps its Markdown parser on stable GFM/CommonMark semantics; math,
// diagram, chart, and sortable-table support is an HTML presentation layer
// applied after the browser receives the body. This module is the single
// entry point so callers never have to know about KaTeX, Mermaid, Vega-Lite,
// or Tablesort individually.
//
// Mermaid and Vega-Lite run before KaTeX so KaTeX never scans raw diagram
// or chart source code. ZenUML diagrams additionally register Mermaid's
// external-diagram plugin before anything initializes — Mermaid Core alone
// does not know the `zenuml` diagram type. Tables sort last so the caller's
// scroll restore lands on the fully enhanced DOM; the sortable-header
// geometry itself is reserved statically in the stylesheet, so the
// enhancement never shifts layout. All runtimes are the shared /runtime/*
// assets the document server embeds, loaded through the runtime loader only
// when the document actually uses them.

import type { ResolvedMode } from "../model";
import { copyText } from "./clipboard";
import {
  ensureZenUMLRegistered,
  loadKatex,
  loadMermaid,
  loadTablesort,
  loadVegaLite,
  type MathAutoRenderDelimiter,
  type MermaidRuntime,
  type TablesortConstructor,
  type VegaEmbedOptions,
  type VegaEmbedResult,
  type VegaEmbedRuntime,
  type VegaLiteHostConfig,
  type VegaLoader,
} from "./runtime-loader";

const COPY_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><rect x="5.25" y="5.25" width="7.25" height="7.25" rx="1.25"></rect><path d="M10.75 5.25V3.5c0-.69-.56-1.25-1.25-1.25H3.5c-.69 0-1.25.56-1.25 1.25v6c0 .69.56 1.25 1.25 1.25h1.75"></path></svg>';
const COPIED_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m3.5 8.25 2.1 2.1 4.9-4.9"></path></svg>';
const COPY_FAILED_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m4.5 4.5 7 7m0-7-7 7"></path></svg>';
const HEADING_ANCHOR_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M4 9h1v1H4c-1.5 0-3-1.69-3-3.5S2.55 3 4 3h4c1.45 0 3 1.69 3 3.5 0 1.41-.91 2.72-2 3.25V8.59c.58-.45 1-1.27 1-2.09C10 5.22 8.98 4 8 4H4c-.98 0-2 1.22-2 2.5S3 9 4 9zm9-3h-1v1h1c1 0 2 1.22 2 2.5S13.98 12 13 12H9c-.98 0-2-1.22-2-2.5 0-.83.42-1.64 1-2.09V6.25c-1.09.53-2 1.84-2 3.25C6 11.31 7.55 13 9 13h4c1.45 0 3-1.69 3-3.5S14.5 6 13 6z"></path></svg>';
// Lucide ZoomIn, verbatim path data. Like the icons above, the body DOM is not
// owned by React, so the magnifier control renders an inline SVG instead of a
// lucide-react component.
const IMAGE_ZOOM_ICON =
  '<svg aria-hidden="true" focusable="false" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><circle cx="11" cy="11" r="8"></circle><path d="m21 21-4.3-4.3"></path><path d="M11 8v6"></path><path d="M8 11h6"></path></svg>';

// "$$" must precede "$" so the inline delimiter does not swallow the block
// delimiter first.
const MATH_DELIMITERS: MathAutoRenderDelimiter[] = [
  { left: "$$", right: "$$", display: true },
  { left: "\\[", right: "\\]", display: true },
  { left: "\\(", right: "\\)", display: false },
  { left: "$", right: "$", display: false },
];

// KaTeX auto-render pairs every two single dollars without Markdown's usual
// flanking rules. That turns prose such as "$9 ... $200" into one enormous
// formula. Literal dollars are split into ignored spans before auto-render;
// valid inline math keeps the Pandoc/VSCode-compatible boundary contract:
// opener followed by non-space, closer preceded by non-space and not followed
// by a digit.
const LITERAL_DOLLAR_CLASS = "m2h-literal-dollar";
const KATEX_IGNORED_CONTENT_SELECTOR = `script, noscript, style, textarea, pre, code, option, .katex, .${LITERAL_DOLLAR_CLASS}`;

// Plain GFM tables Goldmark renders as bare <table>; a class attribute marks
// user-authored HTML tables the client-side sorter must leave untouched, and
// data-m2h-sortable marks tables a previous enhancement pass already owns.
const SORTABLE_TABLE_SELECTOR =
  'table:not([class]):not([data-m2h-sortable="true"])';

// Interactive nodes inside a header cell. A click on such a header must keep
// its native behavior (navigate, focus the control) instead of also sorting.
const INTERACTIVE_HEADER_SELECTOR =
  "a, button, input, select, textarea, [contenteditable=true]";

// Headers Tablesort made sortable: it stamps role=columnheader on every
// header cell and skips data-sort-method="none" ones, so sortable cells are
// the role-bearing cells that were not opted out.
const SORTABLE_HEADER_SELECTOR =
  'thead th[role="columnheader"]:not([data-sort-method="none"])';

// Source lines beyond which a fenced code block collapses behind an expand
// toggle. Counting logical source lines (not browser-wrapped visual lines)
// keeps the threshold stable regardless of reader width.
const CODE_COLLAPSE_LINE_THRESHOLD = 25;

// Fenced code languages whose blocks are replaced by a rendered visual — a
// Mermaid diagram or a Vega-Lite chart — instead of staying a code block.
// Every code presentation enhancement (copy button, line-number gutter,
// collapse fold) skips these blocks: their <pre> is about to be replaced
// wholesale, so a frame, gutter, or toggle would only flash before dying
// with the pre.
const RICH_VISUAL_LANGUAGES = new Set([
  "language-mermaid",
  "language-vega-lite",
  "language-vegalite",
]);

function isRichVisualCode(code: HTMLElement): boolean {
  for (const language of RICH_VISUAL_LANGUAGES) {
    if (code.classList.contains(language)) {
      return true;
    }
  }
  return false;
}

// Monotonic id handed to every collapsed code block so the toggle's
// aria-controls always points at the <pre> it manages.
let codeBlockSequence = 0;

/**
 * Enhance already-rendered Markdown HTML inside `root` by rendering Mermaid
 * diagrams and KaTeX math, making plain GFM tables sortable, and collapsing
 * overlong code blocks behind an expand toggle. Safe to call repeatedly;
 * errors from individual blocks are suppressed so a broken diagram never
 * breaks the whole document.
 *
 * Each runtime is loaded only when the document actually uses it — Mermaid
 * requires a fenced `language-mermaid` block, KaTeX a math delimiter, and
 * Tablesort a plain `<table>` — so a plain document preview never downloads
 * the multi-megabyte diagram runtime.
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
// Options for one rich-content enhancement pass. `isCurrent` is the freshness
// check consulted after every async step (see below); `onVisualError` receives
// a short, reader-facing message for every diagram/chart/image-adjacent visual
// that failed to render — the detailed reason goes to the console, the page
// shows the stable placeholder plus this message in the top warning.
export interface RichContentOptions {
  mode: ResolvedMode;
  isCurrent?: () => boolean;
  onVisualError?: (message: string) => void;
}

export async function renderRichContent(
  root: HTMLElement,
  options: RichContentOptions,
): Promise<void> {
  const { mode, isCurrent, onVisualError } = options;
  addHeadingPermalinks(root);
  addCodeCopyButtons(root);
  addCodeLineNumbers(root);
  addCollapsibleCodeBlocks(root);
  addImageEnhancements(root);
  // Kick the Tablesort download off before awaiting Mermaid/KaTeX so all
  // needed runtimes load in parallel; the tables themselves are enhanced only
  // after those settle. The reserved indicator space is static CSS, so the
  // enhancement itself never changes table geometry. The Vega-Lite trio
  // downloads alongside them the same way.
  const tablesortLoad = hasSortableTables(root) ? loadTablesort() : null;
  const vegaLiteLoad = hasVegaLiteBlocks(root) ? loadVegaLite() : null;
  if (hasMermaidBlocks(root)) {
    const mermaid = await prepareMermaid(mode, hasZenUMLBlocks(root));
    await renderMermaid(mermaid, root, mode, isCurrent, onVisualError);
  }
  if (isCurrent !== undefined && !isCurrent()) {
    return;
  }
  if (vegaLiteLoad !== null) {
    const embed = await vegaLiteLoad;
    if (isCurrent !== undefined && !isCurrent()) {
      return;
    }
    await renderVegaLiteBlocks(embed, root, isCurrent, onVisualError);
  }
  if (isCurrent !== undefined && !isCurrent()) {
    return;
  }
  if (hasMathText(root)) {
    const renderMathInElement = await loadKatex();
    protectLiteralDollars(root);
    renderMathInElement(root, {
      delimiters: MATH_DELIMITERS,
      ignoredClasses: [LITERAL_DOLLAR_CLASS],
      throwOnError: false,
    });
  }
  if (tablesortLoad !== null) {
    const Tablesort = await tablesortLoad;
    if (isCurrent !== undefined && !isCurrent()) {
      return;
    }
    enhanceSortableTables(root, Tablesort);
  }
}

function hasMermaidBlocks(root: HTMLElement): boolean {
  return root.querySelector("pre > code.language-mermaid") !== null;
}

// Vega-Lite charts are fenced blocks whose JSON spec must be self-contained
// (data.values only; the loader below denies every external resource). The
// canonical fence language is `vega-lite`; `vegalite` is accepted as an alias.
const VEGA_LITE_CODE_SELECTOR =
  "pre > code.language-vega-lite, pre > code.language-vegalite";

function hasVegaLiteBlocks(root: HTMLElement): boolean {
  return root.querySelector(VEGA_LITE_CODE_SELECTOR) !== null;
}

// Mirrors the official plugin's own detector (/^\s*zenuml/): a diagram counts
// as ZenUML only when the keyword leads its source, so prose or other
// diagrams that merely mention "zenuml" never trigger the plugin download.
function isZenUMLSource(source: string): boolean {
  return /^\s*zenuml/.test(source);
}

function hasZenUMLBlocks(root: HTMLElement): boolean {
  for (const code of root.querySelectorAll<HTMLElement>(
    "pre > code.language-mermaid",
  )) {
    if (isZenUMLSource(code.textContent ?? "")) {
      return true;
    }
  }
  return false;
}

function hasSortableTables(root: HTMLElement): boolean {
  return root.querySelector(SORTABLE_TABLE_SELECTOR) !== null;
}

// Instantiate the client-side sorter on every plain GFM table with a header
// and more than one body row. Tablesort keeps its sort direction on the
// aria-sort attribute itself; keyboard access, titles, and the explicit
// "none" baseline come from finalize/sync below. The data-m2h-sortable marker
// is set before construction and removed again if construction throws, so a
// failed enhancement can be retried instead of leaving the table marked done.
function enhanceSortableTables(
  root: HTMLElement,
  Tablesort: TablesortConstructor,
): void {
  for (const table of root.querySelectorAll<HTMLTableElement>(
    SORTABLE_TABLE_SELECTOR,
  )) {
    if (
      table.tHead === null ||
      table.tBodies.length === 0 ||
      table.tBodies[0].rows.length <= 1
    ) {
      continue;
    }
    table.dataset.m2hSortable = "true";
    prepareSortableTable(table);
    try {
      new Tablesort(table);
    } catch {
      delete table.dataset.m2hSortable;
      continue;
    }
    finalizeSortableTable(table);
  }
}

// Headers whose cells embed interactive nodes are opted out of sorting before
// construction: Tablesort natively ignores data-sort-method="none", so clicks
// on a link or button header keep their native meaning.
function prepareSortableTable(table: HTMLTableElement): void {
  for (const th of table.querySelectorAll<HTMLTableCellElement>("thead th")) {
    if (th.querySelector(INTERACTIVE_HEADER_SELECTOR) !== null) {
      th.dataset.sortMethod = "none";
    }
  }
}

// Tablesort writes direction to aria-sort itself but only assigns a lowercase
// (inert) `tabindex` property, so keyboard access, the sort hint title, and
// the explicit aria-sort="none" baseline are layered on here.
function finalizeSortableTable(table: HTMLTableElement): void {
  table.addEventListener("afterSort", () => {
    syncTableSortState(table);
  });
  for (const th of table.querySelectorAll<HTMLTableCellElement>(
    SORTABLE_HEADER_SELECTOR,
  )) {
    th.tabIndex = 0;
    th.setAttribute("aria-sort", "none");
    th.title = "点击升序排序";
    th.addEventListener("keydown", (event) => {
      if (event.key !== "Enter" && event.key !== " ") {
        return;
      }
      event.preventDefault();
      th.click();
    });
  }
  syncTableSortState(table);
}

// Keep titles in step with the aria-sort attribute after every sort. When
// another column takes over, Tablesort strips the previous column's
// aria-sort; restoring the explicit "none" baseline keeps the header state
// machine predictable for CSS and assistive technology alike.
function syncTableSortState(table: HTMLTableElement): void {
  for (const th of table.querySelectorAll<HTMLTableCellElement>(
    SORTABLE_HEADER_SELECTOR,
  )) {
    const order = th.getAttribute("aria-sort");
    if (order === "ascending") {
      th.title = "当前升序，点击切换为降序";
    } else if (order === "descending") {
      th.title = "当前降序，点击切换为升序";
    } else {
      th.setAttribute("aria-sort", "none");
      th.title = "点击升序排序";
    }
  }
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

function protectLiteralDollars(root: HTMLElement): void {
  const walker = root.ownerDocument.createTreeWalker(
    root,
    NodeFilter.SHOW_TEXT,
  );
  const candidates: Text[] = [];
  for (let node = walker.nextNode(); node !== null; node = walker.nextNode()) {
    if (node.nodeType === Node.TEXT_NODE) {
      candidates.push(node as Text);
    }
  }

  for (const node of candidates) {
    const parent = node.parentElement;
    const source = node.data;
    if (
      parent === null ||
      !source.includes("$") ||
      parent.closest(KATEX_IGNORED_CONTENT_SELECTOR) !== null
    ) {
      continue;
    }

    const literalIndexes = literalDollarIndexes(source);
    if (literalIndexes.length === 0) {
      continue;
    }

    const fragment = root.ownerDocument.createDocumentFragment();
    let start = 0;
    for (const index of literalIndexes) {
      fragment.append(source.slice(start, index));
      const literal = root.ownerDocument.createElement("span");
      literal.className = LITERAL_DOLLAR_CLASS;
      literal.textContent = "$";
      fragment.append(literal);
      start = index + 1;
    }
    fragment.append(source.slice(start));
    node.replaceWith(fragment);
  }
}

function literalDollarIndexes(source: string): number[] {
  const singles: number[] = [];
  const openers: number[] = [];
  const matched = new Set<number>();

  for (let index = 0; index < source.length; index += 1) {
    if (
      source[index] !== "$" ||
      source[index - 1] === "$" ||
      source[index + 1] === "$"
    ) {
      continue;
    }
    singles.push(index);

    const previous = source[index - 1] ?? "";
    const next = source[index + 1] ?? "";
    const canOpen = next !== "" && !/\s/u.test(next);
    const canClose =
      previous !== "" && !/\s/u.test(previous) && !/[0-9]/u.test(next);

    if (canClose && openers.length > 0) {
      const opener = openers.pop();
      if (opener !== undefined) {
        matched.add(opener);
        matched.add(index);
      }
    } else if (canOpen) {
      openers.push(index);
    }
  }

  return singles.filter((index) => !matched.has(index));
}

// Prepend a GitHub-style permalink anchor to every heading that carries an id.
// The anchor is a bare "#id" fragment so the WebUI link interceptor treats it as
// a same-document target (scroll in place, no reload). It is rendered as an
// inline SVG rather than a React icon because the body DOM lives outside the
// component tree. Idempotent: re-running enhancement on the same body (e.g. a
// server-sent hot-swap that writes the same HTML) must not stack duplicates.
function addHeadingPermalinks(root: HTMLElement): void {
  const headings = root.querySelectorAll<HTMLElement>(
    "h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]",
  );
  for (const heading of headings) {
    const id = heading.id;
    if (id === "" || heading.querySelector(":scope > .m2h-heading-anchor")) {
      continue;
    }
    const anchor = document.createElement("a");
    anchor.className = "m2h-heading-anchor";
    anchor.href = `#${id}`;
    // aria-hidden keeps the icon out of the accessibility tree so it never
    // pollutes the heading's own accessible name (screen-reader users navigate
    // by heading text, and the TOC panel already provides an accessible link to
    // each section). A title still gives sighted users a hover tooltip.
    anchor.setAttribute("aria-hidden", "true");
    anchor.title = "此标题的永久链接";
    anchor.innerHTML = HEADING_ANCHOR_ICON;
    heading.prepend(anchor);
  }
}

// The direct <code> child of a fenced <pre>. The line-number gutter also
// lives directly under the pre, so firstElementChild cannot be trusted once a
// gutter has been prepended.
function directCodeChild(pre: HTMLPreElement): HTMLElement | null {
  const code = pre.querySelector(":scope > code");
  return code instanceof HTMLElement ? code : null;
}

// Wrap a fenced <pre> in the frame that owns the block's external geometry.
// The <pre> stays the code block's only scroll container, while the frame is
// the positioning context for absolutely-positioned overlay controls (copy,
// later the collapse toggle): an absolutely-positioned descendant of a scroll
// container scrolls with the container's content, so the copy button must
// never live inside the scrollport itself. Idempotent, like every enhancement
// here: a pre already framed is returned as-is, so re-running the enhancement
// on the same body stacks nothing.
function ensureCodeFrame(pre: HTMLPreElement): HTMLDivElement {
  const parent = pre.parentElement;
  if (
    parent instanceof HTMLDivElement &&
    parent.classList.contains("m2h-code-frame")
  ) {
    return parent;
  }

  const frame = document.createElement("div");
  frame.className = "m2h-code-frame";
  pre.replaceWith(frame);
  frame.append(pre);
  return frame;
}

function addCodeCopyButtons(root: HTMLElement): void {
  for (const pre of root.querySelectorAll<HTMLPreElement>("pre")) {
    const code = directCodeChild(pre);
    // Rich-visual blocks are skipped: their pass replaces the <pre> with a
    // rendered visual, so a frame and button would only flash before dying
    // with the pre.
    if (code === null || isRichVisualCode(code)) {
      continue;
    }
    const frame = ensureCodeFrame(pre);
    if (frame.querySelector(":scope > .m2h-code-copy") !== null) {
      continue;
    }

    const button = document.createElement("button");
    button.type = "button";
    button.className = "m2h-code-copy";
    button.innerHTML = COPY_ICON;
    button.setAttribute("aria-label", "复制代码");
    button.title = "复制代码";
    button.addEventListener("click", () => {
      void copyText(code.textContent ?? "").then((copied) => {
        setCopyStatus(button, copied);
      });
    });
    frame.append(button);
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

// Fold code blocks longer than CODE_COLLAPSE_LINE_THRESHOLD source lines by
// turning their frame into a collapsible block (the frame gains the
// m2h-code-block modifier and the collapse toggle). Rich-visual blocks are
// skipped: their pass later replaces the <pre> with a rendered container, and
// a visual must never be folded behind a toggle. Idempotent: a pre already
// living in a collapsible frame is left alone, so re-running the enhancement
// on the same body (e.g. a same-HTML hot swap) stacks nothing.
function addCollapsibleCodeBlocks(root: HTMLElement): void {
  for (const pre of root.querySelectorAll<HTMLPreElement>("pre")) {
    const code = directCodeChild(pre);
    if (code === null) {
      continue;
    }
    if (isRichVisualCode(code)) {
      continue;
    }
    if (pre.closest(".m2h-code-block") !== null) {
      continue;
    }

    const lineCount = codeSourceLineCount(code);
    if (lineCount <= CODE_COLLAPSE_LINE_THRESHOLD) {
      continue;
    }

    enhanceCodeBlock(pre, lineCount);
  }
}

// Prepend a gutter of line numbers to every fenced code block. The numbers
// live in their own span beside the <code> element, so the source text — and
// with it the copy control — never contains them, Chroma's token spans stay
// untouched, and the collapse fold keeps treating the whole <pre> as one
// visual unit. Rich-visual blocks are skipped: their <pre> is replaced by a
// rendered visual, so a gutter would only flash before disappearing.
// Idempotent, like the copy control: re-running the enhancement on the same
// body (e.g. a same-HTML hot swap) stacks nothing.
function addCodeLineNumbers(root: HTMLElement): void {
  for (const pre of root.querySelectorAll<HTMLPreElement>("pre")) {
    const code = directCodeChild(pre);
    if (code === null || isRichVisualCode(code)) {
      continue;
    }
    if (pre.querySelector(":scope > .m2h-code-line-numbers") !== null) {
      continue;
    }

    const gutter = document.createElement("span");
    gutter.className = "m2h-code-line-numbers";
    gutter.setAttribute("aria-hidden", "true");
    const lineCount = Math.max(codeSourceLineCount(code), 1);
    for (let line = 1; line <= lineCount; line += 1) {
      const number = document.createElement("span");
      number.textContent = String(line);
      gutter.append(number);
    }

    pre.classList.add("m2h-code-with-lines");
    code.before(gutter);
  }
}

// Count logical source lines, immune to the trailing "\n" fenced code
// characteristically carries (a naive split would report one line too many).
// The collapse fold and the line-number gutter share this single algorithm so
// they can never disagree about a block's line count.
function codeSourceLineCount(code: HTMLElement): number {
  const source = (code.textContent ?? "").replace(/\r\n?/g, "\n");
  if (source === "") {
    return 0;
  }
  return source.split("\n").length - (source.endsWith("\n") ? 1 : 0);
}

// Turn one overlong block's frame into a collapsible block: the frame gains
// the m2h-code-block modifier and the expand/collapse toggle beside the copy
// control, so short and long blocks share one frame structure
// (frame > pre + copy [+ toggle]) instead of nesting a second wrapper.
// Collapsing is purely presentational — the code's text content stays complete
// so the copy control keeps copying the whole block — and only data attributes
// flip on toggle, so the CSS owns the geometry and nothing here re-parses or
// rewrites the code.
function enhanceCodeBlock(pre: HTMLPreElement, lineCount: number): void {
  const frame = ensureCodeFrame(pre);

  frame.classList.add("m2h-code-block");
  frame.dataset.collapsible = "true";
  frame.dataset.collapsed = "true";
  frame.dataset.lineCount = String(lineCount);
  frame.style.setProperty(
    "--m2h-code-collapse-lines",
    String(CODE_COLLAPSE_LINE_THRESHOLD),
  );

  const preID = `m2h-code-block-${++codeBlockSequence}`;
  pre.id = preID;

  const collapsedLabel = `展开代码（共${lineCount}行）`;
  const toggle = document.createElement("button");
  toggle.type = "button";
  toggle.className = "m2h-code-toggle";
  toggle.setAttribute("aria-expanded", "false");
  toggle.setAttribute("aria-controls", preID);
  toggle.textContent = collapsedLabel;
  toggle.addEventListener("click", () => {
    const wasCollapsed = frame.dataset.collapsed !== "false";
    frame.dataset.collapsed = wasCollapsed ? "false" : "true";
    toggle.setAttribute("aria-expanded", wasCollapsed ? "true" : "false");
    toggle.textContent = wasCollapsed ? "折叠代码" : collapsedLabel;
  });
  frame.append(toggle);
}

// Enhance every plain <img> in the body: a wrapper frame carrying a hover
// tooltip with the image's alt text and intrinsic size/format, failure
// tracking that collapses a broken image into the shared placeholder, and —
// when the image is Lightbox-eligible — a magnifier button plus the marker
// the React layer keys its click-time item lookup on. No position index is
// recorded here: DOM order at click time is the only source of truth, so
// another enhancement that reorders the body (a sortable table moving <tr>
// rows) cannot desync the Lightbox. Mermaid never appears here as an <img> —
// its pass turns the source pre into a framed div.mermaid holding an SVG —
// so plain img scanning excludes it for free, and raw-HTML <img> tags are
// covered by the same query. Idempotent like every enhancement: an image
// already inside a frame is skipped, so re-running on the same body stacks
// no second frame, button or tooltip — a failed image keeps its placeholder
// frame and is not re-processed either. The Lightbox is the only gated
// enhancement: a multi-image anchor's per-image frames land inside the link,
// where a <button> would be invalid interactive content, so those images get
// everything except the trigger (see imageLightboxTarget).
function addImageEnhancements(root: HTMLElement): void {
  for (const image of root.querySelectorAll<HTMLImageElement>("img")) {
    if (image.closest(".m2h-image-frame") !== null) {
      continue;
    }

    const frame = ensureImagePresentationFrame(image);
    addImageMetadataTooltip(image, frame);
    if (imageLightboxTarget(image) !== null) {
      addImageLightbox(image, frame);
    }
    trackImageFailure(image);
  }
}

// Wrap one image's visual root in the presentation frame every enhancement
// hangs off. The frame carries the image's external geometry (the pinned
// magnifier, the hover tooltip), so it wraps the element that visually stands
// for the image: the <img> itself, its <picture> when one wraps it, or the
// anchor of a sole-image link — never a multi-image anchor, where each image
// is framed individually inside the link instead.
function ensureImagePresentationFrame(image: HTMLImageElement): HTMLElement {
  const frame = document.createElement("span");
  frame.className = "m2h-image-frame";
  const target = imageVisualRoot(image);
  target.replaceWith(frame);
  frame.append(target);
  return frame;
}

// The frame's hover tooltip: it repeats the alt text and adds the intrinsic
// size and format for sighted readers. aria-hidden: the <img> alt already
// provides the name to assistive technology, so a second accessible copy adds
// nothing. Every image gets one — alt or not — because the size/format line
// is useful on its own.
function addImageMetadataTooltip(
  image: HTMLImageElement,
  frame: HTMLElement,
): void {
  const tooltip = document.createElement("span");
  tooltip.className = "m2h-image-name-tooltip";
  tooltip.setAttribute("aria-hidden", "true");
  const name = image.alt.trim();
  if (name !== "") {
    const alt = document.createElement("span");
    alt.className = "m2h-image-tooltip-alt";
    alt.textContent = name;
    tooltip.append(alt);
  }
  const meta = document.createElement("span");
  meta.className = "m2h-image-tooltip-meta";
  tooltip.append(meta);
  frame.append(tooltip);
  trackImageMetadata(image, meta);
}

// Attach the Lightbox to one framed image: the item marker React's click-time
// lookup keys on, plus the magnifier button beside the visual root inside the
// frame.
function addImageLightbox(image: HTMLImageElement, frame: HTMLElement): void {
  image.dataset.m2hLightboxItem = "true";
  frame.append(createLightboxTrigger("查看大图"));
}

// The placeholder a failed image collapses into. Served from the app's own
// public assets, so the fallback itself failing can only mean the app is
// broken — the m2hFallback marker below makes sure that can never loop.
const IMAGE_FALLBACK_SRC = "/image-load-failed.svg";

// Watch a framed image for a load failure and collapse it into the shared
// placeholder. The replacement lives here, not in React's error reporting:
// the frame, the tooltip, the trigger, and now the failure state are all body
// DOM owned by this layer, while React only surfaces the top warning. Covers
// both orders — the error may arrive after enhancement, or the image may
// already have failed (cached failure, aborted load) before it ever ran.
function trackImageFailure(image: HTMLImageElement): void {
  if (image.complete && image.naturalWidth === 0) {
    replaceImageWithFallback(image);
    return;
  }
  image.addEventListener(
    "error",
    () => {
      replaceImageWithFallback(image);
    },
    { once: true },
  );
}

function replaceImageWithFallback(image: HTMLImageElement): void {
  // The fallback's own failure must never re-enter this path (the marker is
  // also how the React layer tells a placeholder error from a real one).
  if (image.dataset.m2hFallback === "true") {
    return;
  }
  image.dataset.m2hFallback = "true";
  // Keep the failed source around for the top warning to report — after the
  // swap, "src" no longer names what failed.
  image.dataset.m2hOriginalSrc = image.getAttribute("src") ?? "";
  // A <picture>'s <source> elements would keep feeding the original (missing)
  // candidates; without them the <img> src swap takes effect.
  image.parentElement?.querySelectorAll("source").forEach((source) => {
    source.remove();
  });
  image.src = IMAGE_FALLBACK_SRC;
  image.alt = "图片加载失败";
  // A broken image is nothing to magnify: withdraw the Lightbox marker and
  // hide the magnifier, exactly as a visual that never rendered does.
  syncImageLightboxAvailability(image, false);
  // The metadata tooltip describes the picture that was supposed to load; a
  // placeholder has neither a meaningful name nor size/format rows.
  const frame = image.closest(".m2h-image-frame");
  frame?.querySelector(":scope > .m2h-image-name-tooltip")?.remove();
  frame?.classList.add("m2h-image-failed");
}

// The image counterpart of syncRichVisualLightboxAvailability: whether an
// image may open the Lightbox is a function of it showing real content, and
// after every state change the marker and the trigger are brought in line.
function syncImageLightboxAvailability(
  image: HTMLImageElement,
  available: boolean,
): void {
  if (available) {
    image.dataset.m2hLightboxItem = "true";
  } else {
    delete image.dataset.m2hLightboxItem;
  }
  const trigger = image
    .closest(".m2h-image-frame")
    ?.querySelector<HTMLButtonElement>(":scope > .m2h-lightbox-trigger");
  if (trigger) {
    trigger.hidden = !available;
  }
}

// Fill the tooltip's metadata line with the image's intrinsic size and format.
// The enhancement pass usually runs before the browser finished fetching, so
// the first fill happens on the load event; a cached image is already complete
// and reads out immediately. A failed image simply never fills the line — the
// alt row still says what the picture was meant to show.
function trackImageMetadata(
  image: HTMLImageElement,
  meta: HTMLSpanElement,
): void {
  const apply = () => {
    const format = imageFormat(image);
    const size =
      image.naturalWidth > 0
        ? `${image.naturalWidth} × ${image.naturalHeight}`
        : null;
    meta.textContent =
      size === null
        ? (format ?? "")
        : format === null
          ? size
          : `${size} · ${format}`;
  };
  if (image.complete && image.naturalWidth > 0) {
    apply();
    return;
  }
  image.addEventListener("load", apply, { once: true });
}

// The display format of an image, derived from its URL extension or data URL
// MIME — never from a network probe: the bytes are already fetched, so the
// URL is all the metadata that comes for free. JPEG normalizes to "JPG".
export function imageFormat(image: HTMLImageElement): string | null {
  const source = image.currentSrc || image.src;
  if (source === "") {
    return null;
  }
  if (source.startsWith("data:")) {
    const comma = source.indexOf(",");
    const mime = comma === -1 ? source.slice(5) : source.slice(5, comma);
    return imageFormatFromMime(mime);
  }
  try {
    const url = new URL(source, window.location.href);
    const extension = url.pathname.slice(url.pathname.lastIndexOf(".") + 1);
    return url.pathname.includes(".")
      ? imageFormatFromExtension(extension)
      : null;
  } catch {
    return null;
  }
}

function imageFormatFromExtension(extension: string): string | null {
  switch (extension.toLowerCase()) {
    case "jpg":
    case "jpeg":
    case "jpe":
      return "JPG";
    case "png":
      return "PNG";
    case "svg":
    case "svgz":
      return "SVG";
    case "webp":
      return "WEBP";
    case "gif":
      return "GIF";
    case "avif":
      return "AVIF";
    case "":
      return null;
    default:
      return extension.toUpperCase();
  }
}

function imageFormatFromMime(mime: string): string | null {
  // Data URLs may carry parameters ("image/svg+xml;charset=…", the implicit
  // ";base64") — the subtype alone decides the label.
  const subtype = mime
    .toLowerCase()
    .replace(/^image\//, "")
    .split(";")[0];
  if (subtype === "" || subtype === undefined) {
    return null;
  }
  if (subtype === "svg+xml") {
    return "SVG";
  }
  return imageFormatFromExtension(subtype.replace(/[^a-z0-9]/g, ""));
}

// Build the magnifier control every Lightbox frame carries — the image frame
// and the Mermaid frame share one button so their affordance, gating, and
// styling can never drift apart. The body DOM is not owned by React, so the
// trigger renders an inline SVG instead of a lucide-react component; the
// aria label is the only per-kind difference.
function createLightboxTrigger(ariaLabel: string): HTMLButtonElement {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "m2h-lightbox-trigger";
  button.setAttribute("aria-label", ariaLabel);
  button.title = ariaLabel;
  button.innerHTML = IMAGE_ZOOM_ICON;
  return button;
}

// The element the frame should wrap. A <picture>'s <img> has to stay the
// picture's direct child for source selection to work, so the picture itself
// is the visual root. A sole-image anchor becomes the visual root in turn —
// the frame then wraps the anchor, keeping a trigger button a sibling of the
// <a> instead of an interactive element nested inside one. An anchor holding
// several images is not upgraded: each of its images is framed as itself,
// inside the link (see imageLightboxTarget for why that forbids the Lightbox).
function imageVisualRoot(image: HTMLImageElement): HTMLElement {
  let visualRoot: HTMLElement = image;
  if (visualRoot.parentElement instanceof HTMLPictureElement) {
    visualRoot = visualRoot.parentElement;
  }
  const anchor = visualRoot.closest("a");
  if (anchor !== null && anchor.querySelectorAll("img").length === 1) {
    return anchor;
  }
  return visualRoot;
}

// The image's visual root when it may carry a Lightbox, or null when the
// trigger button would have to land inside an anchor: a multi-image anchor's
// frames wrap each image inside the link, and a <button> there would be
// invalid interactive content whose Enter press would follow the link instead
// of pressing the button. Sole-image anchors upgrade to the visual root
// itself, so their button stays a sibling of the <a> — outside the link.
function imageLightboxTarget(image: HTMLImageElement): HTMLElement | null {
  const target = imageVisualRoot(image);
  if (target instanceof HTMLAnchorElement) {
    return target;
  }
  return target.closest("a") === null ? target : null;
}

// Mermaid's official light theme is "default" and dark theme is "dark". The
// module tracks which one is active so `initialize` only runs when the resolved
// theme actually changes — never on every render — while still reconfiguring
// promptly when the user toggles between light and dark.
type MermaidTheme = "default" | "dark";

let currentMermaidTheme: MermaidTheme | null = null;

const SVG_NAMESPACE = "http://www.w3.org/2000/svg";

// @zenuml/core 3.47.2 always returns the same white, self-styled SVG. Mermaid's
// ordinary sequence renderer bakes a dark palette into its SVG, so leaving the
// ZenUML result untouched creates a conspicuous white island on a dark reader.
// Keep the correction inside the returned SVG: every selector is rooted at the
// explicit diagram theme marker, the style travels with Lightbox snapshots,
// and no rule can reach the host document. These colors mirror Mermaid 11's
// dark sequence-diagram surfaces, foregrounds, and strokes rather than inventing
// a separate visual language for the external renderer.
const ZENUML_DARK_THEME_STYLE = `
svg[data-m2h-zenuml-theme="dark"] { color-scheme: dark; }
svg[data-m2h-zenuml-theme="dark"] .frame-border-outer { fill: #d3d3d3; }
svg[data-m2h-zenuml-theme="dark"] .frame-border-inner,
svg[data-m2h-zenuml-theme="dark"] .frame-header-bg,
svg[data-m2h-zenuml-theme="dark"] .participant-box,
svg[data-m2h-zenuml-theme="dark"] .group-title-bg { fill: #1f2020; }
svg[data-m2h-zenuml-theme="dark"] .frame-header-line,
svg[data-m2h-zenuml-theme="dark"] .participant-box,
svg[data-m2h-zenuml-theme="dark"] .participant-icon [fill="currentColor"]:not([stroke]),
svg[data-m2h-zenuml-theme="dark"] .lifeline,
svg[data-m2h-zenuml-theme="dark"] .fragment-border,
svg[data-m2h-zenuml-theme="dark"] .group-outline { stroke: #d3d3d3; }
svg[data-m2h-zenuml-theme="dark"] .frame-title,
svg[data-m2h-zenuml-theme="dark"] .participant-label,
svg[data-m2h-zenuml-theme="dark"] .message-label,
svg[data-m2h-zenuml-theme="dark"] .fragment-label,
svg[data-m2h-zenuml-theme="dark"] .fragment-condition,
svg[data-m2h-zenuml-theme="dark"] .fragment-section-label,
svg[data-m2h-zenuml-theme="dark"] .return-label,
svg[data-m2h-zenuml-theme="dark"] .return-icon,
svg[data-m2h-zenuml-theme="dark"] .group-title-text { fill: #cccccc; }
svg[data-m2h-zenuml-theme="dark"] .participant-icon { color: #cccccc; }
svg[data-m2h-zenuml-theme="dark"] .message-line,
svg[data-m2h-zenuml-theme="dark"] .arrow-head,
svg[data-m2h-zenuml-theme="dark"] .return-line,
svg[data-m2h-zenuml-theme="dark"] .return-arrow,
svg[data-m2h-zenuml-theme="dark"] .arrow-head path[stroke] { stroke: #cccccc; }
svg[data-m2h-zenuml-theme="dark"] .arrow-head:not(.arrow-open) { fill: #cccccc; }
svg[data-m2h-zenuml-theme="dark"] .arrow-open,
svg[data-m2h-zenuml-theme="dark"] .return-arrow { fill: none; }
svg[data-m2h-zenuml-theme="dark"] .occurrence,
svg[data-m2h-zenuml-theme="dark"] .fragment-header { fill: #474949; }
svg[data-m2h-zenuml-theme="dark"] .occurrence { stroke: #d3d3d3; }
svg[data-m2h-zenuml-theme="dark"] .fragment-separator { stroke: #2f2f2f; }
svg[data-m2h-zenuml-theme="dark"] .divider-bg { fill: #2f2f2f; stroke: #aaaa33; }
svg[data-m2h-zenuml-theme="dark"] .divider-label { fill: #d3d3d3; }
svg[data-m2h-zenuml-theme="dark"] .comment-text,
svg[data-m2h-zenuml-theme="dark"] .seq-number { fill: #b8b6b6; }
`;

function applyZenUMLTheme(target: HTMLElement, mode: ResolvedMode): void {
  const svg = target.querySelector<SVGSVGElement>(":scope > svg");
  if (svg === null) {
    return;
  }

  svg.setAttribute("data-m2h-zenuml-theme", mode);
  if (mode === "light") {
    return;
  }

  let definitions = svg.querySelector<SVGDefsElement>(":scope > defs");
  if (definitions === null) {
    definitions = document.createElementNS(SVG_NAMESPACE, "defs");
    svg.insertBefore(definitions, svg.firstChild);
  }
  const style = document.createElementNS(SVG_NAMESPACE, "style");
  style.setAttribute("data-m2h-zenuml-theme-style", "dark");
  style.textContent = ZENUML_DARK_THEME_STYLE;
  definitions.append(style);
}

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

// The single Mermaid preparation path for first render and theme rerender:
// load the runtime, register the ZenUML external-diagram plugin when the
// document needs it, then configure the theme — in that order. Mermaid's
// integration model requires external diagrams to be registered before
// initialize, and registration is attempted on rerender too, so a document
// whose first registration failed (or a reloaded runtime in dev/test) can
// still recover on a theme switch instead of staying broken.
async function prepareMermaid(
  mode: ResolvedMode,
  needsZenUML: boolean,
): Promise<MermaidRuntime> {
  const mermaid = await loadMermaid();
  if (needsZenUML) {
    await ensureZenUMLRegistered(mermaid);
  }
  ensureMermaidInitialized(mode, mermaid);
  return mermaid;
}

// Render one diagram offscreen via mermaid.render and swap the resulting SVG in
// atomically, so the container never flashes back to source text while the new
// palette resolves. Returns false when the render is no longer current, telling
// the caller to abort the remaining targets; a render that throws leaves any
// existing SVG in place so one broken diagram never breaks the document.
// Whether the Lightbox is offered is decided by the SVG's real presence after
// every paint attempt (see syncMermaidLightboxAvailability).
async function paintMermaidTarget(
  mermaid: MermaidRuntime,
  target: HTMLElement,
  source: string,
  mode: ResolvedMode,
  isCurrent?: () => boolean,
  onVisualError?: (message: string) => void,
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
    if (isZenUMLSource(source)) {
      applyZenUMLTheme(target, mode);
    }
    result.bindFunctions?.(target);
  } catch (error) {
    // A single bad diagram is isolated. The first paint failed: the container
    // still shows the raw Mermaid source, which collapses into the shared
    // placeholder — the page never advertises engine source text, and a
    // parser's "Syntax error in text …" transcript stays in the console.
    // A failed repaint (an older SVG sits in the container) keeps that SVG:
    // one broken theme switch must not erase what the reader is looking at.
    if (target.querySelector("svg") === null) {
      renderRichVisualFailure(target, "mermaid", onVisualError);
    } else {
      onVisualError?.(RICH_VISUAL_FAILURE_TITLE.mermaid);
    }
    console.warn("Failed to render Mermaid diagram", {
      diagramType: getMermaidDiagramType(source),
      error,
    });
  }
  syncRichVisualLightboxAvailability(target);
  return true;
}

// The first word of a diagram source names its diagram type. Reported with
// render failures so "unknown diagram type" (an unregistered external
// diagram) is distinguishable from a fetch failure or a syntax error.
function getMermaidDiagramType(source: string): string {
  const type = source.trimStart().split(/\s+/, 1)[0];
  return type === "" ? "unknown" : type;
}

// The rich visuals share one failure contract with failed images: the slot
// collapses into the app-owned placeholder instead of keeping raw engine
// source text (a Mermaid grammar, a chart's JSON) on the page, and only a
// short stable title names what failed. Parser details stay in the console —
// they are diagnostics, not reader content.
type RichVisualKind = "mermaid" | "vega-lite";

const RICH_VISUAL_FAILURE_TITLE: Record<RichVisualKind, string> = {
  mermaid: "Mermaid 图表渲染失败",
  "vega-lite": "Vega-Lite 图表渲染失败",
};

// Replace a visual container's content with the shared failure UI and report
// the stable message through the caller's onVisualError. Used on every
// first-paint failure; a failed *repaint* (an older SVG already sits in the
// container) keeps that SVG instead — one broken theme switch must not erase
// a diagram the reader is looking at.
function renderRichVisualFailure(
  target: HTMLElement,
  kind: RichVisualKind,
  onVisualError?: (message: string) => void,
): void {
  const message = RICH_VISUAL_FAILURE_TITLE[kind];
  const container = document.createElement("div");
  container.className = "m2h-rich-visual-error";
  const title = document.createElement("div");
  title.className = "m2h-rich-visual-error-title";
  title.textContent = message;
  const image = document.createElement("img");
  image.src = IMAGE_FALLBACK_SRC;
  image.alt = "";
  image.setAttribute("aria-hidden", "true");
  container.append(title, image);
  target.replaceChildren(container);
  onVisualError?.(message);
}

// Whether a visual may open the Lightbox is a function of its rendered SVG,
// never of the frame's existence. After every paint attempt — success or
// failure — the marker and the trigger are brought in line with the SVG's
// presence: a visual that never rendered offers no magnifier (a click would
// snapshot nothing), while a failed theme repaint keeps the previous SVG and
// with it a still-working Lightbox. This is deliberately more correct than
// deleting the trigger on failure: it covers the first-render and re-render
// cases with one rule. Shared by every rich-visual engine: the container
// sits inside a m2h-rich-visual-frame and holds an SVG exactly when its
// engine's last paint succeeded.
function syncRichVisualLightboxAvailability(target: HTMLElement): void {
  const available = target.querySelector("svg") !== null;

  if (available) {
    target.dataset.m2hLightboxItem = "true";
  } else {
    delete target.dataset.m2hLightboxItem;
  }

  const trigger = target
    .closest(".m2h-rich-visual-frame")
    ?.querySelector<HTMLButtonElement>(":scope > .m2h-lightbox-trigger");

  if (trigger) {
    trigger.hidden = !available;
  }
}

async function renderMermaid(
  mermaid: MermaidRuntime,
  root: HTMLElement,
  mode: ResolvedMode,
  isCurrent?: () => boolean,
  onVisualError?: (message: string) => void,
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
    // No Lightbox marker here: the container starts with nothing to show, and
    // syncMermaidLightboxAvailability stamps the marker — on the container,
    // never the SVG inside, whose markup every repaint rewrites — only once a
    // paint has really produced an SVG.
    mermaidSources.set(container, source);

    // A stable frame around the diagram. The container's content is owned by
    // mermaid.render (and replaced wholesale on theme switches), so the
    // Lightbox trigger can never live inside it; the frame keeps the trigger
    // — and any focus on it — alive across every repaint. The trigger starts
    // hidden: until the first paint succeeds there is nothing to enlarge.
    // The frame carries the shared rich-visual class plus the engine-specific
    // one, so shared presentation (trigger hover) never has to enumerate each
    // visual engine while engine-specific geometry stays addressable.
    const trigger = createLightboxTrigger("查看 Mermaid 图表");
    trigger.hidden = true;
    const frame = document.createElement("div");
    frame.className = "m2h-rich-visual-frame m2h-mermaid-frame";
    pre.replaceWith(frame);
    frame.append(container, trigger);
    targets.push(container);
  }

  for (const target of targets) {
    const source = mermaidSources.get(target);
    if (source === undefined) {
      continue;
    }
    if (
      !(await paintMermaidTarget(
        mermaid,
        target,
        source,
        mode,
        isCurrent,
        onVisualError,
      ))
    ) {
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
  onVisualError?: (message: string) => void,
): Promise<void> {
  const targets = Array.from(
    root.querySelectorAll<HTMLElement>(".mermaid"),
  ).filter((target) => mermaidSources.has(target));

  if (targets.length === 0) {
    return;
  }

  const needsZenUML = targets.some((target) => {
    const source = mermaidSources.get(target);
    return source !== undefined && isZenUMLSource(source);
  });
  const mermaid = await prepareMermaid(mode, needsZenUML);

  for (const target of targets) {
    const source = mermaidSources.get(target);
    if (source === undefined) {
      continue;
    }
    if (
      !(await paintMermaidTarget(
        mermaid,
        target,
        source,
        mode,
        isCurrent,
        onVisualError,
      ))
    ) {
      return;
    }
  }
}

// Each chart's JSON source, retained like the Mermaid sources in a WeakMap:
// the spec text can be long, and keeping it out of data-* attributes lets a
// theme re-render (see the Vega-Lite rerender in the theme path) re-embed
// without re-parsing the document body.
const vegaLiteSources = new WeakMap<HTMLElement, string>();

// The live embed result per chart container. Unlike Mermaid, a Vega view
// registers timers and document-level event listeners that only finalize()
// detaches, so every re-embed and every document teardown must finalize the
// previous result or the view leaks.
const vegaLiteViews = new WeakMap<HTMLElement, VegaEmbedResult>();

// Finalize every chart view still alive under `root`. Called before the body
// DOM is wholesale replaced (document switch, hot swap, unmount): after that
// the containers are unreachable, and with them the views recorded here.
export function finalizeVegaLiteViews(root: HTMLElement): void {
  for (const container of root.querySelectorAll<HTMLElement>(
    ".m2h-vega-lite",
  )) {
    const result = vegaLiteViews.get(container);
    if (result !== undefined) {
      result.finalize();
      vegaLiteViews.delete(container);
    }
  }
}

// Host-controlled loader that denies every external fetch. Vega calls it in
// three contexts: dataflow (data.url, string config and patch), image (mark
// images), and href (hyperlink navigation). The first two are refused
// outright — charts stay self-contained in the WebUI and exported HTML
// alike, the same contract with or without a page CSP — while a hyperlink
// click is navigation, not a fetch: it resolves through the same reader-wide
// policy as Markdown links. Cross-origin HTTP(S) opens a new tab with
// noopener; same-origin keeps the browser default; every other scheme
// (javascript:, data:, …) is refused, so a spec can never turn a chart mark
// into script execution. Vega writes every key of the sanitized result as an
// attribute on the anchor it synthesizes for the click, which is how the
// target/rel policy reaches the navigation.
function denyExternalResource(): Promise<never> {
  return Promise.reject(
    new Error("external Vega-Lite data loading is not supported"),
  );
}

function sanitizeHyperlink(
  uri: string,
): Promise<{ href: string; target?: string; rel?: string }> {
  let resolved: URL;
  try {
    resolved = new URL(uri, window.location.href);
  } catch {
    return denyExternalResource();
  }
  if (resolved.protocol !== "http:" && resolved.protocol !== "https:") {
    return denyExternalResource();
  }
  if (resolved.origin === window.location.origin) {
    return Promise.resolve({ href: resolved.href });
  }
  return Promise.resolve({
    href: resolved.href,
    target: "_blank",
    rel: "noopener noreferrer",
  });
}

const denyNetworkLoader: VegaLoader = {
  sanitize: (uri, options) =>
    options?.context === "href"
      ? sanitizeHyperlink(uri)
      : denyExternalResource(),
  load: () => denyExternalResource(),
};

// The embed options are host policy, never document content. Vega-Embed
// merges spec.usermeta.embedOptions over the caller's options, so leaving
// that key in place would let a Markdown document reopen the actions menu,
// switch the renderer, or aim editorUrl/loader at another origin. The rest
// of usermeta is author data and stays untouched.
function withoutEmbedOptions(
  spec: Record<string, unknown>,
): Record<string, unknown> {
  const usermeta = spec.usermeta;
  if (
    typeof usermeta !== "object" ||
    usermeta === null ||
    !("embedOptions" in usermeta)
  ) {
    return spec;
  }
  const sanitized: Record<string, unknown> = {
    ...spec,
    usermeta: { ...(usermeta as Record<string, unknown>) },
  };
  delete (sanitized.usermeta as Record<string, unknown>).embedOptions;
  return sanitized;
}

// Parse one chart's JSON spec. Returns null when the source is not a JSON
// object, so the failure isolates to this single block; the warning carries
// the same shape as the render failures below to keep the console uniform.
function parseVegaLiteSpec(source: string): Record<string, unknown> | null {
  let parsed: unknown;
  try {
    parsed = JSON.parse(source);
  } catch {
    console.warn("Failed to render Vega-Lite chart", {
      error: new Error("Vega-Lite specification must be valid JSON"),
    });
    return null;
  }
  if (typeof parsed !== "object" || parsed === null || Array.isArray(parsed)) {
    console.warn("Failed to render Vega-Lite chart", {
      error: new Error("Vega-Lite specification must be a JSON object"),
    });
    return null;
  }
  return parsed as Record<string, unknown>;
}

// The URL of a Vega-Lite data reference, or null when `value` is not a data
// reference that points at a URL. Data references are objects with a `url`
// string; `values`, `name`, `graticule` and `sequence` sources carry none.
function dataRefUrl(value: unknown): string | null {
  if (typeof value !== "object" || value === null || Array.isArray(value)) {
    return null;
  }
  const url = (value as Record<string, unknown>).url;
  return typeof url === "string" && url !== "" ? url : null;
}

// Spec keys whose values are arrays of nested unit or composition specs.
const COMPOSITE_SPEC_KEYS = ["layer", "concat", "hconcat", "vconcat"];

// The first external data URL in a Vega-Lite spec, or null. Only the data
// source positions Vega itself reads are inspected — the top-level `data`,
// every entry of `datasets`, the `from.data` of lookup transforms, and the
// same positions inside composed children (layer/concat/hconcat/vconcat and
// the `spec` child of facet/repeat). A `url` key anywhere else (mark config,
// descriptions, usermeta, the image/href encoding channels) is not a data
// position and stays untouched: those flow through the loader or the link
// policy instead of this preflight.
function firstExternalDataUrl(spec: Record<string, unknown>): string | null {
  const direct = dataRefUrl(spec.data);
  if (direct !== null) {
    return direct;
  }
  const datasets = spec.datasets;
  if (typeof datasets === "object" && datasets !== null) {
    for (const dataset of Object.values(datasets)) {
      const url = dataRefUrl(dataset);
      if (url !== null) {
        return url;
      }
    }
  }
  const transform = spec.transform;
  if (Array.isArray(transform)) {
    for (const entry of transform) {
      if (typeof entry !== "object" || entry === null) {
        continue;
      }
      const from = (entry as Record<string, unknown>).from;
      if (typeof from !== "object" || from === null) {
        continue;
      }
      const url = dataRefUrl((from as Record<string, unknown>).data);
      if (url !== null) {
        return url;
      }
    }
  }
  for (const key of COMPOSITE_SPEC_KEYS) {
    const children = spec[key];
    if (!Array.isArray(children)) {
      continue;
    }
    for (const child of children) {
      if (typeof child !== "object" || child === null || Array.isArray(child)) {
        continue;
      }
      const url = firstExternalDataUrl(child as Record<string, unknown>);
      if (url !== null) {
        return url;
      }
    }
  }
  const child = spec.spec;
  if (typeof child === "object" && child !== null && !Array.isArray(child)) {
    const url = firstExternalDataUrl(child as Record<string, unknown>);
    if (url !== null) {
      return url;
    }
  }
  return null;
}

// Reject unsupported external data before the embed. The host loader already
// denies every fetch, but Vega swallows a loader rejection internally and the
// embed resolves with an empty, mark-less SVG that looks like success — it
// would even open the Lightbox on a chart that never drew. Turning the same
// data positions into an up-front failure gives the ordinary isolated-chart
// contract instead: source kept, no SVG, no Lightbox trigger, one warning.
function assertSelfContainedSpec(spec: Record<string, unknown>): void {
  const url = firstExternalDataUrl(spec);
  if (url !== null) {
    throw new Error(
      `Vega-Lite specification must be self-contained: external data.url (${url}) is not supported`,
    );
  }
}

// One reader-theme color, resolved from the live stylesheet so the CSS
// variables stay the single source of truth (the chart chrome follows the
// toolbar theme for free). The fallback covers environments where custom
// properties do not resolve (tests, exotic embedders); each pair mirrors the
// corresponding :root / .dark value in index.css.
function themeColor(variable: string, light: string, dark: string): string {
  const resolved = getComputedStyle(document.documentElement)
    .getPropertyValue(variable)
    .trim();
  if (resolved !== "") {
    return resolved;
  }
  return document.documentElement.classList.contains("dark") ? dark : light;
}

// The chart palette m2h overlays onto every spec: reader chrome only —
// background, axis/legend/title text, grid and stroke colors. The author's
// mark colors and scale ranges are never touched, so a spec's data semantics
// survive a theme switch unchanged; Vega's own named themes stay unused
// (upstream marks them experimental). Colors read as computed CSS values so
// `Markdown reader theme → CSS variables → Vega config` has one source.
function vegaLiteThemeConfig(): VegaLiteHostConfig {
  const foreground = themeColor(
    "--foreground",
    "oklch(0.145 0 0)",
    "oklch(0.985 0 0)",
  );
  const muted = themeColor(
    "--muted-foreground",
    "oklch(0.556 0 0)",
    "oklch(0.708 0 0)",
  );
  return {
    // null = transparent: the chart sits on the reader's page background,
    // so a dark document never gets a white chart slab.
    background: null,
    axis: {
      labelColor: foreground,
      titleColor: foreground,
      gridColor: muted,
      domainColor: muted,
      tickColor: muted,
    },
    legend: {
      labelColor: foreground,
      titleColor: foreground,
    },
    title: {
      color: foreground,
    },
    view: {
      stroke: null,
    },
  };
}

// Build the host policy every embed call carries. `mode` is pinned to
// "vega-lite" — without it Vega-Embed infers from $schema and silently falls
// back to raw Vega when both are absent — the renderer is pinned to SVG (the
// vector pipeline Mermaid already uses: theme-aware, Lightbox-serializable),
// and the actions menu (Export/Source/Editor) stays off. `ast` routes
// expression evaluation through the interpreter Vega-Embed bundles, keeping
// charts alive under the strict CSP (script-src 'self', no 'unsafe-eval').
function vegaLiteEmbedOptions(): VegaEmbedOptions {
  return {
    mode: "vega-lite",
    renderer: "svg",
    actions: false,
    ast: true,
    tooltip: true,
    loader: denyNetworkLoader,
    config: vegaLiteThemeConfig(),
  };
}

// Embed one chart and, like the Mermaid paints, isolate its failure: a chart
// that fails gets the shared placeholder in place of its JSON source (never
// raw spec text on the page) and no Lightbox marker, and never breaks the
// next chart. A re-embed finalizes the previous view first — Vega views hold
// timers and document-level listeners that leak without it. A failed re-embed
// of a chart that already shows an SVG keeps that SVG — one broken theme
// switch must not erase what the reader is looking at. Returns false when the
// render is no longer current, telling the caller to abort the remaining
// targets.
async function embedVegaLiteTarget(
  embed: VegaEmbedRuntime,
  target: HTMLElement,
  source: string,
  isCurrent?: () => boolean,
  onVisualError?: (message: string) => void,
): Promise<boolean> {
  const previous = vegaLiteViews.get(target);
  if (previous !== undefined) {
    previous.finalize();
    vegaLiteViews.delete(target);
  }
  try {
    const spec = parseVegaLiteSpec(source);
    if (spec !== null) {
      assertSelfContainedSpec(spec);
      const result: VegaEmbedResult = await embed(
        target,
        withoutEmbedOptions(spec),
        vegaLiteEmbedOptions(),
      );
      if (isCurrent !== undefined && !isCurrent()) {
        result.finalize();
        return false;
      }
      vegaLiteViews.set(target, result);
    } else {
      // The source never parsed: nothing will embed, so the container keeps
      // showing the JSON unless the failure UI replaces it here.
      if (target.querySelector("svg") === null) {
        renderRichVisualFailure(target, "vega-lite", onVisualError);
      }
    }
  } catch (error) {
    // A failure that struck before Vega-Embed touched the container (e.g. a
    // compile error) leaves the old SVG visible and stays; one that struck
    // mid-render clears the container and collapses into the placeholder —
    // the JSON source never reappears as page content.
    if (target.querySelector("svg") === null) {
      renderRichVisualFailure(target, "vega-lite", onVisualError);
    } else {
      onVisualError?.(RICH_VISUAL_FAILURE_TITLE["vega-lite"]);
    }
    console.warn("Failed to render Vega-Lite chart", { error });
  }
  syncRichVisualLightboxAvailability(target);
  return true;
}

async function renderVegaLiteBlocks(
  embed: VegaEmbedRuntime,
  root: HTMLElement,
  isCurrent?: () => boolean,
  onVisualError?: (message: string) => void,
): Promise<void> {
  const targets: HTMLElement[] = [];
  for (const code of root.querySelectorAll<HTMLElement>(
    VEGA_LITE_CODE_SELECTOR,
  )) {
    const pre = code.parentElement;
    if (!(pre instanceof HTMLPreElement)) {
      continue;
    }
    const source = code.textContent ?? "";
    const container = document.createElement("div");
    container.className = "m2h-vega-lite";
    container.textContent = source;
    // No Lightbox marker yet: syncRichVisualLightboxAvailability stamps it
    // only once an embed has really produced an SVG.
    vegaLiteSources.set(container, source);

    // The same stable frame contract as Mermaid: the container's content is
    // owned by vegaEmbed (and replaced wholesale on re-embeds), so the
    // trigger survives every repaint outside the container. It starts
    // hidden — until the first embed succeeds there is nothing to enlarge.
    const trigger = createLightboxTrigger("查看 Vega-Lite 图表");
    trigger.hidden = true;
    const frame = document.createElement("div");
    frame.className = "m2h-rich-visual-frame m2h-vega-lite-frame";
    pre.replaceWith(frame);
    frame.append(container, trigger);
    targets.push(container);
  }

  for (const target of targets) {
    const source = vegaLiteSources.get(target);
    if (source === undefined) {
      continue;
    }
    if (
      !(await embedVegaLiteTarget(
        embed,
        target,
        source,
        isCurrent,
        onVisualError,
      ))
    ) {
      return;
    }
  }
}

// Re-embed only existing charts in the new theme, leaving the rest of the
// document body untouched — the chart twin of rerenderMermaid. The previous
// view is finalized inside embedVegaLiteTarget before each re-embed, and a
// chart whose first render failed gets its retry here, mirroring the Mermaid
// recovery model.
async function rerenderVegaLite(
  root: HTMLElement,
  isCurrent?: () => boolean,
  onVisualError?: (message: string) => void,
): Promise<void> {
  const targets = Array.from(
    root.querySelectorAll<HTMLElement>(".m2h-vega-lite"),
  ).filter((target) => vegaLiteSources.has(target));

  if (targets.length === 0) {
    return;
  }

  const embed = await loadVegaLite();
  for (const target of targets) {
    const source = vegaLiteSources.get(target);
    if (source === undefined) {
      continue;
    }
    if (
      !(await embedVegaLiteTarget(
        embed,
        target,
        source,
        isCurrent,
        onVisualError,
      ))
    ) {
      return;
    }
  }
}

// The single theme-switch entry point for every rich visual: Mermaid bakes
// its official palette into the SVG and Vega-Lite reads the reader theme's
// CSS variables into its config, so both engines re-render on a light/dark
// toggle while paragraphs, KaTeX, and copy controls keep their DOM identity
// and focus. Callers stay engine-agnostic — this layer owns which visuals
// are theme-sensitive.
export async function rerenderThemeSensitiveContent(
  root: HTMLElement,
  options: RichContentOptions,
): Promise<void> {
  const { mode, isCurrent, onVisualError } = options;
  await rerenderMermaid(root, mode, isCurrent, onVisualError);
  await rerenderVegaLite(root, isCurrent, onVisualError);
}
