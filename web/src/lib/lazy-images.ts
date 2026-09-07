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
// reports — and handed back by the viewport observer once the image may load.

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

// Load margin, not visibility: waiting for the viewport itself means a fast
// scroll always shows the placeholder before the pixels. Images start loading
// roughly half a screen to a screen before they can be seen, which is still
// lazy loading for a reader — the document below the reading position never
// loads at all.
const VIEWPORT_MARGIN = "512px 0px";

// Settle notifications, so the presentation layer (tooltip, Lightbox) can
// follow the state machine without this module knowing about it.
export interface LazyImageHooks {
  onImageSettled?: (image: HTMLImageElement, loaded: boolean) => void;
}

export interface LazyImageController {
  disconnect(): void;
}

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

// Watch a mounted body's pending images and load each one as it approaches
// the viewport. The returned controller ends the observation when the body it
// belongs to is replaced. Without IntersectionObserver there is no scheduling
// left to do — every pending image restores right away instead of sitting on
// the placeholder forever.
export function observeLazyImages(
  root: HTMLElement,
  hooks: LazyImageHooks = {},
): LazyImageController {
  const images = Array.from(
    root.querySelectorAll<HTMLImageElement>(PENDING_SELECTOR),
  );
  if (!("IntersectionObserver" in window)) {
    for (const image of images) {
      startLazyLoad(image, null, hooks);
    }
    return { disconnect() {} };
  }
  const observer = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (
          !entry.isIntersecting ||
          !(entry.target instanceof HTMLImageElement)
        ) {
          continue;
        }
        startLazyLoad(entry.target, observer, hooks);
      }
    },
    { root: null, rootMargin: VIEWPORT_MARGIN, threshold: 0 },
  );
  for (const image of images) {
    observer.observe(image);
  }
  return {
    disconnect() {
      observer.disconnect();
    },
  };
}

// Move one image from pending to loading and hand it its sources back. The
// observer only unobserves on settle: a load already in flight keeps loading
// when the reader scrolls past it, and a second intersection callback is
// absorbed by the state guard.
function startLazyLoad(
  image: HTMLImageElement,
  observer: { unobserve(image: HTMLImageElement): void } | null,
  hooks: LazyImageHooks,
): void {
  if (image.dataset.m2hLazyState !== "pending") {
    return;
  }
  // The failure pipeline may have collapsed this slot into its placeholder
  // before the image ever approached the viewport; restoring would overwrite
  // that placeholder with the source that already failed.
  if (image.dataset.m2hFallback === "true") {
    return;
  }
  image.dataset.m2hLazyState = "loading";
  image.addEventListener(
    "load",
    () => {
      settleLazyImage(image, true, observer, hooks);
    },
    { once: true },
  );
  image.addEventListener(
    "error",
    () => {
      settleLazyImage(image, false, observer, hooks);
    },
    { once: true },
  );
  restoreImageSources(image);
  // A cached image can finish synchronously with the restore; the load event
  // still fires, but settling here keeps the state machine honest for
  // engines that might not deliver one after a same-task src change.
  if (image.complete && image.naturalWidth > 0) {
    settleLazyImage(image, true, observer, hooks);
  }
}

function settleLazyImage(
  image: HTMLImageElement,
  loaded: boolean,
  observer: { unobserve(image: HTMLImageElement): void } | null,
  hooks: LazyImageHooks,
): void {
  // The placeholder's own load event (a pending delivery racing the restore)
  // must not settle the image, and a settled image only leaves the machine
  // once — both funnel into the same state guard.
  if (image.dataset.m2hLazyState !== "loading") {
    return;
  }
  image.dataset.m2hLazyState = loaded ? "loaded" : "failed";
  image.removeAttribute("aria-busy");
  observer?.unobserve(image);
  hooks.onImageSettled?.(image, loaded);
}

// Give one image its real sources back. The restore order matters for
// <picture>: the <source> candidates must be in place before the <img> src
// lands, because assigning src can trigger resource selection immediately
// and would otherwise pick from a candidate list that is still parked.
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
