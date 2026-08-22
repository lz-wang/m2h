// Snapshot collection for the image Lightbox.
//
// The Lightbox must never hold references to the article's <img> elements: the
// body DOM is wholesale replaced on every document load and hot swap
// (root.innerHTML = html), so a live element reference would go stale the
// moment the server sends new HTML. Instead, opening the Lightbox snapshots
// each enhanced image's display-relevant attributes; the component then works
// purely from data, which also keeps a body refresh free to close it.

export interface LightboxImage {
  src: string;
  srcSet: string | null;
  sizes: string | null;
  alt: string;
  title: string | null;
}

// The opened Lightbox: the body's image snapshots plus the image being viewed.
export interface LightboxState {
  images: LightboxImage[];
  index: number;
}

// Snapshot the body's Lightbox-enhanced images in *current* DOM order and
// locate the pressed trigger's image among them.
//
// The index is deliberately resolved here rather than baked into the DOM at
// enhancement time: a sortable table really moves <tr> elements around, so any
// position recorded when the triggers were injected can go stale and address
// the wrong image. Querying at click time keeps the Lightbox decoupled from
// every other enhancement that may reorder the body.
//
// Returns null when the selected image is not one of the root's enhanced
// images (or has left the tree); the caller then simply does not open.
//
// currentSrc (resolved against the document URL, and the srcset winner when
// one applies) is preferred over the raw attribute so the Lightbox reuses
// exactly the resource the browser already loaded for the body.
export function collectLightboxState(
  root: HTMLElement,
  selectedImage: HTMLImageElement,
): LightboxState | null {
  const elements = Array.from(
    root.querySelectorAll<HTMLImageElement>(
      'img[data-m2h-lightbox-image="true"]',
    ),
  );
  const index = elements.indexOf(selectedImage);
  if (index === -1) {
    return null;
  }
  return {
    index,
    images: elements.map((image) => ({
      src: image.currentSrc || image.src,
      srcSet: image.getAttribute("srcset"),
      sizes: image.getAttribute("sizes"),
      alt: image.alt,
      title: image.title || null,
    })),
  };
}
