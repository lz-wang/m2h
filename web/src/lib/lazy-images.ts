// Lazy document images.
//
// The Markdown body is mounted by handing server HTML to the DOM, and the
// moment that happens the browser's resource scanner starts fetching every
// <img> in it — so any post-mount fix (setting loading="lazy", swapping src,
// registering an observer) races requests that already began. The rewrite
// therefore has to happen before the HTML ever touches the live document:
// prepareLazyImages parses the server HTML inside a <template>, whose content
// is inert and invisible to the resource scanner, and replaces every image's
// network sources with the shared placeholder. The real sources are parked in
// data attributes — the same data-m2h-original-src the failure path already
// reports — and handed back once the image may load.

// Served from the app's public assets under the /ui/ mount the document
// server exposes them at, like the failure placeholder.
export const IMAGE_LOADING_SRC = "/ui/image-loading.svg";

// One lazy image's lifecycle, kept in a single attribute so every consumer
// (observer, enhancement layer, tests) reads the same source of truth:
//
//   pending → loading → loaded
//                  └──→ failed
export type LazyImageState = "pending" | "loading" | "loaded" | "failed";

const PENDING_SELECTOR = 'img[data-m2h-lazy-state="pending"]';

// Parse server HTML into an inert fragment with every image rewritten to the
// loading placeholder. Images that request nothing (no src, no srcset) stay
// untouched — rewriting them would start the very request lazy loading
// exists to avoid. The returned fragment is live DOM detached from the
// document; callers mount it with replaceChildren in the same task, before
// the browser gets a chance to observe anything.
export function prepareLazyImages(html: string): DocumentFragment {
  const template = document.createElement("template");
  template.innerHTML = html;
  for (const image of template.content.querySelectorAll<HTMLImageElement>(
    "img",
  )) {
    prepareLazyImage(image);
  }
  return template.content;
}

function prepareLazyImage(image: HTMLImageElement): void {
  const src = image.getAttribute("src");
  const srcset = image.getAttribute("srcset");
  if ((src === null || src === "") && srcset === null) {
    return;
  }
  // The parked sources double as the failure report's "original URL": both
  // this rewrite and a later placeholder swap keep the attribute pointing at
  // the image the author referenced.
  if (src !== null) {
    image.dataset.m2hOriginalSrc = src;
  }
  if (srcset !== null) {
    image.dataset.m2hOriginalSrcset = srcset;
    image.removeAttribute("srcset");
  }
  // A <picture>'s <source> candidates bypass the <img> src entirely, so they
  // are parked with the same treatment — media/type/sizes stay put, only the
  // request-issuing attribute moves. The failure path drops <source> elements
  // for the same reason.
  image
    .closest("picture")
    ?.querySelectorAll<HTMLSourceElement>("source")
    .forEach((source) => {
      const sourceSrcset = source.getAttribute("srcset");
      if (sourceSrcset !== null) {
        source.dataset.m2hOriginalSrcset = sourceSrcset;
        source.removeAttribute("srcset");
      }
    });
  image.dataset.m2hLazyState = "pending";
  image.setAttribute("aria-busy", "true");
  image.src = IMAGE_LOADING_SRC;
}

// Give every pending image inside root its real sources back, marking it
// loading. The restore order matters for <picture>: the <source> candidates
// must be in place before the <img> src lands, because assigning src can
// trigger resource selection immediately and would otherwise pick from a
// candidate list that is still parked.
export function restoreLazyImages(root: HTMLElement): void {
  for (const image of root.querySelectorAll<HTMLImageElement>(
    PENDING_SELECTOR,
  )) {
    restoreImageSources(image);
    image.dataset.m2hLazyState = "loading";
  }
}

function restoreImageSources(image: HTMLImageElement): void {
  image
    .closest("picture")
    ?.querySelectorAll<HTMLSourceElement>("source")
    .forEach(restoreSource);
  const srcset = image.dataset.m2hOriginalSrcset;
  if (srcset !== undefined) {
    image.setAttribute("srcset", srcset);
    delete image.dataset.m2hOriginalSrcset;
  }
  const src = image.dataset.m2hOriginalSrc;
  if (src !== undefined) {
    image.src = src;
    delete image.dataset.m2hOriginalSrc;
  }
}

function restoreSource(source: HTMLSourceElement): void {
  const srcset = source.dataset.m2hOriginalSrcset;
  if (srcset !== undefined) {
    source.setAttribute("srcset", srcset);
    delete source.dataset.m2hOriginalSrcset;
  }
}
