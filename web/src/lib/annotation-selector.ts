// Selector construction: DOM Selection/Range → versioned text selector.
//
// The canonical reading text is the concatenation of every content text
// node of the rendered body in DOM order, measured in Unicode code points
// (never UTF-16 units). m2h-inserted decorations (line-number gutters,
// copy buttons, heading anchors, collapse toggles, lightbox triggers,
// image metadata) are excluded via the data-m2h-decor marker the
// rich-content renderer stamps on everything it inserts, and generated
// visuals (.katex, svg) are excluded wholesale. Tabbed panels are included
// whether or not their tab is active — the canonical text must be
// deterministic, so a tab switch never shifts positions; only the visible
// panel's text is physically selectable.

export interface TextQuote {
  exact: string;
  prefix: string;
  suffix: string;
}

export interface TextPosition {
  start: number;
  end: number;
}

export interface AnnotationSelector {
  quote: TextQuote;
  position: TextPosition;
  headingId?: string;
  tabId?: string;
  blockHint?: string;
  version: 1;
}

export const SELECTOR_VERSION = 1 as const;

// Selector context length (in code points) captured around the quote.
export const SELECTOR_CONTEXT_RUNES = 64;

// Regions the canonical text never enters: m2h's own decorations, KaTeX
// output and inline SVG. Everything else is reader content.
const EXCLUDED_SELECTOR = "[data-m2h-decor], .katex, svg";

interface TextSegment {
  node: Text;
  // Code point offset of this node's first character in the canonical text.
  startCP: number;
  runes: number;
}

// TextIndex maps between DOM Ranges and code-point spans of the canonical
// reading text of one rendered body.
export class TextIndex {
  readonly root: Element;
  readonly text: string;
  private readonly segments: TextSegment[];

  constructor(root: Element) {
    this.root = root;
    this.segments = [];
    let offset = 0;
    let text = "";
    const walker = document.createTreeWalker(
      root,
      NodeFilter.SHOW_TEXT,
      acceptContentNode,
    );
    let node = walker.nextNode();
    while (node !== null) {
      const segment = node as Text;
      const runes = codePointCount(segment.data);
      if (runes > 0) {
        this.segments.push({ node: segment, startCP: offset, runes });
        text += segment.data;
        offset += runes;
      }
      node = walker.nextNode();
    }
    this.text = text;
  }

  get length(): number {
    return this.segments.reduce((total, segment) => total + segment.runes, 0);
  }

  // spanFromRange converts a DOM range into a code-point span. Selections
  // that touch excluded regions or collapse to nothing return null.
  spanFromRange(range: Range): TextPosition | null {
    const start = this.boundaryFrom(range.startContainer, range.startOffset);
    const end = this.boundaryFrom(range.endContainer, range.endOffset);
    if (start === null || end === null || end <= start) {
      return null;
    }
    return { start, end };
  }

  private boundaryFrom(container: Node, utf16Offset: number): number | null {
    if (!(container instanceof Text)) {
      return null;
    }
    const segment = this.segmentFor(container);
    if (segment === null) {
      return null;
    }
    const clamped = Math.min(Math.max(utf16Offset, 0), container.data.length);
    return segment.startCP + codePointsBefore(container.data, clamped);
  }

  private segmentFor(node: Text): TextSegment | null {
    for (const segment of this.segments) {
      if (segment.node === node) {
        return segment;
      }
    }
    return null;
  }

  // rangeFromSpan maps a code-point span back onto a live DOM range. The
  // body must be the one this index was built from — a stale index returns
  // null once its nodes left the document.
  rangeFromSpan(position: TextPosition): Range | null {
    if (
      position.start < 0 ||
      position.end > this.length ||
      position.end <= position.start
    ) {
      return null;
    }
    const start = this.resolveBoundary(position.start);
    const end = this.resolveBoundary(position.end);
    if (start === null || end === null) {
      return null;
    }
    const range = document.createRange();
    try {
      range.setStart(start.node, start.utf16);
      range.setEnd(end.node, end.utf16);
    } catch {
      return null;
    }
    return range;
  }

  private resolveBoundary(cp: number): { node: Text; utf16: number } | null {
    let remaining = cp;
    for (const segment of this.segments) {
      if (remaining <= segment.runes) {
        return {
          node: segment.node,
          utf16: utf16OffsetAt(segment.node.data, remaining),
        };
      }
      remaining -= segment.runes;
    }
    // Offsets landing exactly on the end map to the last segment's end.
    const last = this.segments.at(-1);
    if (last !== undefined && remaining === 0 && cp > 0) {
      return { node: last.node, utf16: last.node.data.length };
    }
    return null;
  }

  // slice returns the canonical text between two code-point offsets.
  slice(start: number, end: number): string {
    return cpSlice(this.text, start, end);
  }

  // context returns up to contextRunes characters around a span.
  context(
    position: TextPosition,
    contextRunes = SELECTOR_CONTEXT_RUNES,
  ): Pick<TextQuote, "prefix" | "suffix"> {
    return {
      prefix: cpSlice(
        this.text,
        Math.max(0, position.start - contextRunes),
        position.start,
      ),
      suffix: cpSlice(
        this.text,
        position.end,
        Math.min(this.length, position.end + contextRunes),
      ),
    };
  }

  // findAll returns the code-point start of every occurrence of needle in
  // the canonical text.
  findAll(needle: string): number[] {
    if (needle === "") {
      return [];
    }
    const starts: number[] = [];
    let from = 0;
    for (;;) {
      const found = this.text.indexOf(needle, from);
      if (found === -1) {
        break;
      }
      starts.push(utf16ToCodePoint(this.text, found));
      from = found + 1;
    }
    return starts;
  }
}

function acceptContentNode(node: Node): number {
  const parent = node.parentElement;
  if (parent === null) {
    return NodeFilter.FILTER_REJECT;
  }
  if (parent.closest(EXCLUDED_SELECTOR) !== null) {
    return NodeFilter.FILTER_REJECT;
  }
  return NodeFilter.FILTER_ACCEPT;
}

// codePointsBefore counts the code points in data starting before a UTF-16
// offset. A surrogate pair straddling the offset counts as one code point
// (the boundary snaps after it): browser selections never split pairs.
export function codePointsBefore(data: string, utf16Offset: number): number {
  let count = 0;
  let index = 0;
  while (index < utf16Offset && index < data.length) {
    const codePoint = data.codePointAt(index);
    if (codePoint === undefined) {
      break;
    }
    index += codePoint > 0xffff ? 2 : 1;
    count += 1;
  }
  return count;
}

// utf16OffsetAt converts a code-point offset inside data back to a UTF-16
// index.
export function utf16OffsetAt(data: string, cp: number): number {
  let index = 0;
  let count = 0;
  while (count < cp && index < data.length) {
    const codePoint = data.codePointAt(index);
    if (codePoint === undefined) {
      break;
    }
    index += codePoint > 0xffff ? 2 : 1;
    count += 1;
  }
  return index;
}

function codePointCount(data: string): number {
  let count = 0;
  for (let index = 0; index < data.length; ) {
    const codePoint = data.codePointAt(index);
    if (codePoint === undefined) {
      break;
    }
    index += codePoint > 0xffff ? 2 : 1;
    count += 1;
  }
  return count;
}

function utf16ToCodePoint(data: string, utf16Offset: number): number {
  return codePointsBefore(data, utf16Offset);
}

function cpSlice(data: string, startCP: number, endCP: number): string {
  if (endCP <= startCP) {
    return "";
  }
  const start = utf16OffsetAt(data, Math.max(0, startCP));
  const end = utf16OffsetAt(data, endCP);
  return data.slice(start, end);
}

// selectorFromRange builds the versioned selector for one selection. It
// returns null when the range does not address canonical content (empty,
// collapsed, or touching excluded regions).
export function selectorFromRange(
  index: TextIndex,
  range: Range,
): AnnotationSelector | null {
  const position = index.spanFromRange(range);
  if (position === null) {
    return null;
  }
  const exact = index.slice(position.start, position.end);
  if (exact.trim() === "") {
    return null;
  }
  const context = index.context(position);
  const selector: AnnotationSelector = {
    quote: { exact, ...context },
    position,
    version: SELECTOR_VERSION,
  };
  const anchor = range.startContainer.parentElement;
  if (anchor !== null) {
    const headingId = nearestHeadingId(index, anchor);
    if (headingId !== undefined) {
      selector.headingId = headingId;
    }
    const panel = anchor.closest<HTMLElement>(".m2h-tabbed-panel");
    if (panel?.dataset.m2hTabId !== undefined) {
      selector.tabId = panel.dataset.m2hTabId;
    }
    const block = anchor.closest("pre, table, blockquote");
    if (block !== null) {
      selector.blockHint = block.tagName.toLowerCase();
    }
  }
  return selector;
}

// nearestHeadingId finds the id of the heading governing the annotated
// anchor: the heading containing it, or the nearest preceding heading in
// document order within the indexed root.
export function nearestHeadingId(
  index: TextIndex,
  anchor: Element,
): string | undefined {
  const containing = anchor.closest(
    "h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]",
  );
  if (containing !== null) {
    return containing.id;
  }
  let best: string | undefined;
  for (const heading of index.root.querySelectorAll<HTMLElement>(
    "h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]",
  )) {
    if (
      heading.compareDocumentPosition(anchor) & Node.DOCUMENT_POSITION_FOLLOWING
    ) {
      best = heading.id;
    }
  }
  return best;
}
