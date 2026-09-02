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
// SVG visuals are snapshotted as a serialized SVG data URL: the Lightbox
// renders everything through <img> — the element type its fit, rotation, and
// pan-clamp algorithms are built around — and the SVG stays vector-sharp at
// any zoom level.

// What a snapshot came from. SVG visuals (Mermaid diagrams, Vega-Lite charts)
// serialize into a data URL; only a bitmap image keeps its original src.
export type LightboxItemKind = "image" | "mermaid" | "vega-lite";

// One browsable visual item. `kind` records what the snapshot came from so
// callers (and tests) can distinguish a bitmap image from an SVG visual
// without sniffing the src scheme.
export interface LightboxItem {
  kind: LightboxItemKind;
  src: string;
  srcSet: string | null;
  sizes: string | null;
  alt: string;
  title: string | null;
}

// The opened Lightbox: the body's item snapshots plus the item being viewed.
export interface LightboxState {
  items: LightboxItem[];
  index: number;
}

// The marker the enhancement layer stamps on every Lightbox-eligible element
// (an <img> or a rendered .mermaid container). One marker for both kinds keeps
// the click-time collection a single query in DOM order.
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
  return snapshotSVGVisual(element, "mermaid", "Mermaid 图表");
}

// Serialize a rendered SVG visual — a Mermaid diagram or a Vega-Lite chart —
// into a self-contained SVG data URL. Both engines bake their theme palette
// into the markup at render time, so the snapshot keeps the exact colors the
// reader is looking at. They also commonly size the SVG with percentages
// ("width=100%"), which has no intrinsic size once the SVG stands alone as an
// <img>; the viewBox carries the visual's true geometry, so the clone is
// pinned to its pixel dimensions instead.
function snapshotSVGVisual(
  container: HTMLElement,
  kind: Exclude<LightboxItemKind, "image">,
  alt: string,
): LightboxItem | null {
  const svg = container.querySelector("svg");
  if (svg === null) {
    return null;
  }
  const clone = svg.cloneNode(true);
  const viewBox = (svg.getAttribute("viewBox") ?? "").trim().split(/[\s,]+/);
  if (
    clone instanceof SVGElement &&
    viewBox.length === 4 &&
    viewBox.every((value) => Number.isFinite(Number(value)))
  ) {
    clone.setAttribute("width", viewBox[2]);
    clone.setAttribute("height", viewBox[3]);
  }
  // encodeURIComponent keeps '#' (every theme color) and '<'/'>' from
  // truncating or breaking the data URL.
  const markup = new XMLSerializer().serializeToString(clone);
  return {
    kind,
    src: `data:image/svg+xml;charset=utf-8,${encodeURIComponent(markup)}`,
    srcSet: null,
    sizes: null,
    alt,
    title: null,
  };
}
