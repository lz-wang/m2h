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
  viewBox: string;
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
    const snapshot = snapshotLightboxItem(element);
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

function snapshotLightboxItem(element: HTMLElement): LightboxItem | null {
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
    return snapshotSVGVisual(element, "vega-lite", "Vega-Lite 图表");
  }
  return snapshotSVGVisual(element, "mermaid", "Mermaid 图表");
}

interface Size {
  width: number;
  height: number;
}

function getSVGIntrinsicSize(svg: SVGSVGElement): Size {
  const viewBox = svg.viewBox.baseVal;
  if (viewBox.width > 0 && viewBox.height > 0) {
    return { width: viewBox.width, height: viewBox.height };
  }

  const width = svg.width.baseVal.value;
  const height = svg.height.baseVal.value;
  if (width > 0 && height > 0) {
    return { width, height };
  }

  const rect = svg.getBoundingClientRect();
  if (rect.width > 0 && rect.height > 0) {
    return { width: rect.width, height: rect.height };
  }

  return { width: 1, height: 1 };
}

// Snapshot only the rendered SVG element. Mermaid and Vega-Lite bake the
// active palette into this markup, preserving it across a body replacement.
function snapshotSVGVisual(
  container: HTMLElement,
  kind: Exclude<LightboxItemKind, "image">,
  alt: string,
): LightboxItem | null {
  const svg = container.querySelector<SVGSVGElement>("svg");
  if (svg === null) {
    return null;
  }
  const { width, height } = getSVGIntrinsicSize(svg);
  const markup = new XMLSerializer().serializeToString(svg);
  return {
    kind,
    markup,
    viewBox: svg.getAttribute("viewBox") ?? `0 0 ${width} ${height}`,
    intrinsicWidth: width,
    intrinsicHeight: height,
    alt,
    title: null,
  };
}
