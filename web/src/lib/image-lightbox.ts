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

// Collect every Lightbox-enhanced image of the current body in document order.
// The order matches the data-m2h-lightbox-index values render-rich-content.ts
// assigned, so the index a trigger carries addresses the same image here.
//
// currentSrc (resolved against the document URL, and the srcset winner when
// one applies) is preferred over the raw attribute so the Lightbox reuses
// exactly the resource the browser already loaded for the body.
export function collectLightboxImages(root: HTMLElement): LightboxImage[] {
  return Array.from(
    root.querySelectorAll<HTMLImageElement>(
      'img[data-m2h-lightbox-image="true"]',
    ),
    (image) => ({
      src: image.currentSrc || image.src,
      srcSet: image.getAttribute("srcset"),
      sizes: image.getAttribute("sizes"),
      alt: image.alt,
      title: image.title || null,
    }),
  );
}
