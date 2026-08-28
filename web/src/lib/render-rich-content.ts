// Rich-content enhancement for rendered Markdown HTML.
//
// m2h keeps its Markdown parser on stable GFM/CommonMark semantics; math,
// diagram, and sortable-table support is an HTML presentation layer applied
// after the browser receives the body. This module is the single entry point
// so callers never have to know about KaTeX, Mermaid, or Tablesort
// individually.
//
// Mermaid runs before KaTeX so KaTeX never scans raw diagram source code.
// Tables sort last so the caller's scroll restore lands on the fully enhanced
// DOM; the sortable-header geometry itself is reserved statically in the
// stylesheet, so the enhancement never shifts layout. All runtimes are the
// shared /runtime/* assets the document server embeds, loaded through the
// runtime loader only when the document actually uses them.

import type { ResolvedMode } from "../model";
import { copyText } from "./clipboard";
import {
  loadKatex,
  loadMermaid,
  loadTablesort,
  type MathAutoRenderDelimiter,
  type MermaidRuntime,
  type TablesortConstructor,
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
export async function renderRichContent(
  root: HTMLElement,
  mode: ResolvedMode,
  isCurrent?: () => boolean,
): Promise<void> {
  addHeadingPermalinks(root);
  addCodeCopyButtons(root);
  addCodeLineNumbers(root);
  addCollapsibleCodeBlocks(root);
  addImageLightboxTriggers(root);
  // Kick the Tablesort download off before awaiting Mermaid/KaTeX so all
  // needed runtimes load in parallel; the tables themselves are enhanced only
  // after those settle. The reserved indicator space is static CSS, so the
  // enhancement itself never changes table geometry.
  const tablesortLoad = hasSortableTables(root) ? loadTablesort() : null;
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
    // Mermaid is skipped: its pass replaces the <pre> with a rendered diagram,
    // so a frame and button would only flash before dying with the pre.
    if (code === null || code.classList.contains("language-mermaid")) {
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
// m2h-code-block modifier and the collapse toggle). Mermaid blocks are
// skipped: the Mermaid pass later replaces their <pre> with a rendered
// container, and a diagram must never be folded behind a toggle. Idempotent:
// a pre already living in a collapsible frame is left alone, so re-running
// the enhancement on the same body (e.g. a same-HTML hot swap) stacks nothing.
function addCollapsibleCodeBlocks(root: HTMLElement): void {
  for (const pre of root.querySelectorAll<HTMLPreElement>("pre")) {
    const code = directCodeChild(pre);
    if (code === null) {
      continue;
    }
    if (code.classList.contains("language-mermaid")) {
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
// visual unit. Mermaid blocks are skipped: their <pre> is replaced by a
// rendered diagram, so a gutter would only flash before disappearing.
// Idempotent, like the copy control: re-running the enhancement on the same
// body (e.g. a same-HTML hot swap) stacks nothing.
function addCodeLineNumbers(root: HTMLElement): void {
  for (const pre of root.querySelectorAll<HTMLPreElement>("pre")) {
    const code = directCodeChild(pre);
    if (code === null || code.classList.contains("language-mermaid")) {
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

// Give every plain <img> in the body a Lightbox trigger: a magnifier button
// pinned to the top-right of a wrapper frame, plus the marker the React layer
// keys its click-time item lookup on. The frame also carries a hover tooltip
// with the image's alt text, so a sighted reader sees the Markdown image name
// without opening the Lightbox. No position index is recorded here:
// DOM order at click time is the only source of truth, so another enhancement
// that reorders the body (a sortable table moving <tr> rows) cannot desync
// the Lightbox. Mermaid never appears here as an <img> — its pass turns the
// source pre into a framed div.mermaid holding an SVG — so plain img scanning
// excludes it for free, and raw-HTML <img> tags are covered by the same
// query. Idempotent like every enhancement: an image already carrying the
// marker is skipped, so re-running on the same body stacks no second frame,
// button or tooltip. An anchor holding several images is left entirely alone
// (see imageVisualRoot): raw-HTML semantics win over the enhancement.
function addImageLightboxTriggers(root: HTMLElement): void {
  for (const image of root.querySelectorAll<HTMLImageElement>("img")) {
    if (image.dataset.m2hLightboxItem === "true") {
      continue;
    }
    const target = imageVisualRoot(image);
    if (target === null) {
      continue;
    }
    image.dataset.m2hLightboxItem = "true";

    const frame = document.createElement("span");
    frame.className = "m2h-image-frame";
    target.replaceWith(frame);

    frame.append(target);
    // The tooltip repeats the alt text for sighted readers. aria-hidden: the
    // <img> alt already provides the name to assistive technology, so a
    // second accessible copy adds nothing.
    const name = image.alt.trim();
    if (name !== "") {
      const tooltip = document.createElement("span");
      tooltip.className = "m2h-image-name-tooltip";
      tooltip.textContent = name;
      tooltip.setAttribute("aria-hidden", "true");
      frame.append(tooltip);
    }
    frame.append(createLightboxTrigger("查看大图"));
  }
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

// The element the frame should wrap, or null when the image must not be
// enhanced at all. A <picture>'s <img> has to stay the picture's direct child
// for source selection to work, so the picture itself is the visual root. A
// sole-image anchor becomes the visual root in turn — the frame then wraps
// the anchor, keeping the trigger button a sibling of the <a> instead of an
// interactive element nested inside one. An anchor holding several images
// returns null: it keeps its raw-HTML structure untouched rather than growing
// an invalid <a><button> nesting (whose Enter press would also follow the
// link instead of pressing the button).
function imageVisualRoot(image: HTMLImageElement): HTMLElement | null {
  let visualRoot: HTMLElement = image;
  if (visualRoot.parentElement instanceof HTMLPictureElement) {
    visualRoot = visualRoot.parentElement;
  }
  const anchor = visualRoot.closest("a");
  if (anchor === null) {
    return visualRoot;
  }
  return anchor.querySelectorAll("img").length === 1 ? anchor : null;
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
    // The Lightbox marker rides on the container, never on the SVG inside:
    // paintMermaidTarget rewrites the container's innerHTML on every theme
    // switch, so the marker — like the frame below — survives each repaint.
    container.dataset.m2hLightboxItem = "true";
    mermaidSources.set(container, source);

    // A stable frame around the diagram. The container's content is owned by
    // mermaid.render (and replaced wholesale on theme switches), so the
    // Lightbox trigger can never live inside it; the frame keeps the trigger
    // — and any focus on it — alive across every repaint.
    const frame = document.createElement("div");
    frame.className = "m2h-mermaid-frame";
    pre.replaceWith(frame);
    frame.append(container, createLightboxTrigger("查看 Mermaid 图表"));
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
