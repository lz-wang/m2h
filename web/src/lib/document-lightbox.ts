// Snapshot collection for the document Lightbox.
//
// The Lightbox browses the document's visual items — enhanced images and
// rendered SVG visuals (Mermaid diagrams) — and must never hold references to
// the article's live elements: the body DOM is wholesale replaced on every
// document load and hot swap (root.innerHTML = html), so a live element
// reference would go stale the moment the server sends new HTML. Instead,
// opening the Lightbox snapshots each item's display-relevant data; the
// component then works purely from data, which also keeps a body refresh free
// to close it.
//
// SVG visuals retain their serialized markup. The Lightbox can therefore mount
// them as inline SVG and change their rendered dimensions without routing a
// diagram through an <img> compositor layer.

export type LightboxItemKind = "image" | "mermaid" | "vega-lite";

interface LightboxItemBase {
  alt: string;
  title: string | null;
}

export interface ImageLightboxItem extends LightboxItemBase {
  kind: "image";
  src: string;
  srcSet: string | null;
  sizes: string | null;
}

export interface SVGLightboxItem extends LightboxItemBase {
  kind: "mermaid" | "vega-lite";
  markup: string;
  intrinsicWidth: number;
  intrinsicHeight: number;
}

// The discriminant deliberately prevents an SVG visual from accidentally
// returning to the bitmap <img> rendering path.
export type LightboxItem = ImageLightboxItem | SVGLightboxItem;

// The opened Lightbox: the body's item snapshots plus the item being viewed.
export interface LightboxState {
  items: LightboxItem[];
  index: number;
}

// The marker the enhancement layer stamps on every Lightbox-eligible element
// (an <img> or a rendered visual container — a Mermaid diagram or a Vega-Lite
// chart). One marker for all kinds keeps the click-time collection a single
// query in DOM order.
const LIGHTBOX_ITEM_SELECTOR = '[data-m2h-lightbox-item="true"]';

// Snapshot the body's Lightbox items in *current* DOM order and locate the
// pressed trigger's item among them.
//
// The index is deliberately resolved here rather than baked into the DOM at
// enhancement time: a sortable table really moves <tr> elements around, so any
// position recorded when the triggers were injected can go stale and address
// the wrong item. Querying at click time keeps the Lightbox decoupled from
// every other enhancement that may reorder the body.
//
// Returns null when the selected item is not one of the root's enhanced items
// (or has left the tree), or when it has nothing to show — a Mermaid container
// whose diagram never rendered; the caller then simply does not open.
//
// currentSrc (resolved against the document URL, and the srcset winner when
// one applies) is preferred over the raw attribute so the Lightbox reuses
// exactly the resource the browser already loaded for the body.
export function collectLightboxState(
  root: HTMLElement,
  selectedItem: HTMLElement,
): LightboxState | null {
  const elements = Array.from(
    root.querySelectorAll<HTMLElement>(LIGHTBOX_ITEM_SELECTOR),
  );
  if (elements.indexOf(selectedItem) === -1) {
    return null;
  }
  const items: LightboxItem[] = [];
  let index = -1;
  for (const element of elements) {
    const snapshot = snapshotLightboxItem(element, items.length);
    if (snapshot === null) {
      continue;
    }
    if (element === selectedItem) {
      index = items.length;
    }
    items.push(snapshot);
  }
  return index === -1 ? null : { items, index };
}

function snapshotLightboxItem(
  element: HTMLElement,
  snapshotIndex: number,
): LightboxItem | null {
  if (element instanceof HTMLImageElement) {
    return {
      kind: "image",
      src: element.currentSrc || element.src,
      srcSet: element.getAttribute("srcset"),
      sizes: element.getAttribute("sizes"),
      alt: element.alt,
      title: element.title || null,
    };
  }
  // Non-image items are the enhanced visual containers; their class names
  // the engine, which picks the snapshot's kind and alt text.
  if (element.classList.contains("m2h-vega-lite")) {
    return snapshotSVGVisual(
      element,
      "vega-lite",
      "Vega-Lite 图表",
      snapshotIndex,
    );
  }
  return snapshotSVGVisual(element, "mermaid", "Mermaid 图表", snapshotIndex);
}

interface Size {
  width: number;
  height: number;
}

// Intrinsic length from a width/height attribute. Only unitless and px values
// describe the intrinsic size; percentages and other units say nothing about
// it, so they read as unknown (0) and the caller falls through to measurement.
function getSVGLengthAttribute(svg: SVGSVGElement, name: string): number {
  const raw = svg.getAttribute(name);
  if (raw === null) {
    return 0;
  }
  const match = /^\s*([\d.]+)\s*(px)?\s*$/i.exec(raw);
  return match === null ? 0 : Number(match[1]);
}

function getSVGIntrinsicSize(svg: SVGSVGElement): Size {
  const viewBox = svg.viewBox.baseVal;
  if (viewBox.width > 0 && viewBox.height > 0) {
    return { width: viewBox.width, height: viewBox.height };
  }

  const width = getSVGLengthAttribute(svg, "width");
  const height = getSVGLengthAttribute(svg, "height");
  if (width > 0 && height > 0) {
    return { width, height };
  }

  const rect = svg.getBoundingClientRect();
  if (rect.width > 0 && rect.height > 0) {
    return { width: rect.width, height: rect.height };
  }

  return { width: 1, height: 1 };
}

// Take ownership of the snapshot's root viewport geometry. Renderers emit
// their own sizing — Mermaid's useMaxWidth ships `width="100%"` plus an inline
// `max-width: <diagram-width>px`, which inline beats any stylesheet rule and
// would pin the diagram to its natural size inside an ever-growing Lightbox
// box. The Lightbox alone decides the viewport: the intrinsic size lives in
// viewBox (added when a renderer omitted it), and every viewport-affecting
// style is pinned with !important on the snapshot itself so no future
// renderer's inline or scoped styles can reclaim it.
function normalizeSVGSnapshotGeometry(svg: SVGSVGElement, size: Size): void {
  if (!svg.hasAttribute("viewBox")) {
    svg.setAttribute("viewBox", `0 0 ${size.width} ${size.height}`);
  }
  svg.removeAttribute("width");
  svg.removeAttribute("height");
  svg.style.setProperty("width", "100%", "important");
  svg.style.setProperty("height", "100%", "important");
  svg.style.setProperty("max-width", "none", "important");
  svg.style.setProperty("max-height", "none", "important");
  svg.style.setProperty("min-width", "0", "important");
  svg.style.setProperty("min-height", "0", "important");
}

function rewriteSVGIDReferences(
  svg: SVGSVGElement,
  snapshotIndex: number,
): SVGSVGElement {
  const snapshot = svg.cloneNode(true) as SVGSVGElement;
  const ids = new Map<string, string>();
  const elements = [
    snapshot,
    ...Array.from(snapshot.querySelectorAll<SVGElement>("*")),
  ];
  for (const element of elements) {
    const id = element.id;
    if (id.length > 0) {
      ids.set(id, `m2h-lightbox-${snapshotIndex}-${id}`);
    }
  }
  if (ids.size === 0) {
    return snapshot;
  }

  const rewriteURLReferences = (value: string) =>
    value.replace(
      /url\(\s*(['"]?)#([^\s)'"\]]+)\1\s*\)/g,
      (match, quote, id) => {
        const replacement = ids.get(id);
        return replacement === undefined
          ? match
          : `url(${quote}#${replacement}${quote})`;
      },
    );
  const rewriteFragmentReference = (value: string) => {
    const replacement = ids.get(value.slice(1));
    return value.startsWith("#") && replacement !== undefined
      ? `#${replacement}`
      : value;
  };
  const rewriteIDList = (value: string) =>
    value
      .split(/\s+/)
      .map((id) => ids.get(id) ?? id)
      .join(" ");

  // CSS selectors scope styles to the renamed identifiers too: Mermaid emits
  // `<style>` rules keyed on the root SVG id (`#m2h-mermaid-1 { … }`,
  // `#m2h-mermaid-1 .node { … }`), which silently stop matching once the root
  // is namespaced. Longest-first alternation plus a trailing boundary keeps a
  // suffix id from matching inside a longer one.
  const escapedIDs = [...ids.keys()]
    .sort((a, b) => b.length - a.length)
    .map((id) => id.replace(/[.*+?^${}()|[\]\\]/g, "\\$&"));
  const cssIDPattern =
    escapedIDs.length > 0
      ? new RegExp(`#(${escapedIDs.join("|")})(?![\\w-])`, "g")
      : null;
  const rewriteCSSIDSelectors = (value: string) =>
    cssIDPattern === null
      ? value
      : value.replace(cssIDPattern, (match, id: string) => {
          const replacement = ids.get(id);
          return replacement === undefined ? match : `#${replacement}`;
        });

  for (const element of elements) {
    if (element.id.length > 0) {
      element.id = ids.get(element.id) ?? element.id;
    }
    for (const attribute of Array.from(element.attributes)) {
      const { name, value } = attribute;
      let rewritten = rewriteURLReferences(value);
      if (name === "href" || name === "xlink:href") {
        rewritten = rewriteFragmentReference(rewritten);
      } else if (name === "aria-labelledby" || name === "aria-describedby") {
        rewritten = rewriteIDList(rewritten);
      }
      if (rewritten !== value) {
        element.setAttribute(name, rewritten);
      }
    }
  }
  for (const style of snapshot.querySelectorAll("style")) {
    const rewritten = rewriteCSSIDSelectors(
      rewriteURLReferences(style.textContent ?? ""),
    );
    if (rewritten !== style.textContent) {
      style.textContent = rewritten;
    }
  }
  return snapshot;
}

// Embedded links stay visually intact but inert: the snapshot is a visual,
// and navigation belongs to the body document. tabindex keeps keyboard focus
// out of the Lightbox's embedded links; the component blocks the actual
// navigation on click while leaving the rest of the SVG hittable.
function neutralizeEmbeddedLinks(svg: SVGSVGElement): void {
  for (const anchor of svg.querySelectorAll("a")) {
    anchor.setAttribute("tabindex", "-1");
  }
}

// Snapshot only the rendered SVG element. Mermaid and Vega-Lite bake the
// active palette into this markup, preserving it across a body replacement.
// Rewriting local identifiers keeps the inline copy isolated from its source
// SVG and every other snapshot in the document.
function snapshotSVGVisual(
  container: HTMLElement,
  kind: Exclude<LightboxItemKind, "image">,
  alt: string,
  snapshotIndex: number,
): LightboxItem | null {
  const svg = container.querySelector<SVGSVGElement>("svg");
  if (svg === null) {
    return null;
  }
  const { width, height } = getSVGIntrinsicSize(svg);
  const snapshot = rewriteSVGIDReferences(svg, snapshotIndex);
  normalizeSVGSnapshotGeometry(snapshot, { width, height });
  neutralizeEmbeddedLinks(snapshot);
  const markup = new XMLSerializer().serializeToString(snapshot);
  return {
    kind,
    markup,
    intrinsicWidth: width,
    intrinsicHeight: height,
    alt,
    title: null,
  };
}
