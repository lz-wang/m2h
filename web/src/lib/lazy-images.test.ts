import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import {
  IMAGE_LOADING_SRC,
  observeLazyImages,
  prepareLazyImages,
} from "./lazy-images";

// jsdom has no IntersectionObserver; the controllable fake records what the
// module configures and lets tests deliver intersection rounds by hand.
class FakeIntersectionObserver {
  static instances: FakeIntersectionObserver[] = [];

  callback: IntersectionObserverCallback;
  options: IntersectionObserverInit | undefined;
  observed: Element[] = [];
  disconnected = false;

  constructor(
    callback: IntersectionObserverCallback,
    options?: IntersectionObserverInit,
  ) {
    this.callback = callback;
    this.options = options;
    FakeIntersectionObserver.instances.push(this);
  }

  observe(target: Element): void {
    this.observed.push(target);
  }

  unobserve(target: Element): void {
    this.observed = this.observed.filter((observed) => observed !== target);
  }

  disconnect(): void {
    this.disconnected = true;
    this.observed = [];
  }

  intersect(targets: Element[]): void {
    this.callback(
      targets.map((target) => ({
        isIntersecting: true,
        target,
      })) as IntersectionObserverEntry[],
      this as unknown as IntersectionObserver,
    );
  }
}

// Mount a prepared fragment the way the App does — the images it contains
// went through the same inert rewrite the live body sees.
function mount(html: string): HTMLElement {
  const root = document.createElement("div");
  root.append(prepareLazyImages(html));
  return root;
}

function imageIn(root: HTMLElement): HTMLImageElement {
  const image = root.querySelector("img");
  if (!(image instanceof HTMLImageElement)) {
    throw new Error("prepared image was not mounted");
  }
  return image;
}

describe("prepareLazyImages", () => {
  it("rewrites a plain image to the placeholder before it is mounted", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);

    expect(image.getAttribute("src")).toBe(IMAGE_LOADING_SRC);
    // The real URL is parked for the restore, the state machine starts at
    // pending, and the placeholder announces itself as busy.
    expect(image.dataset.m2hOriginalSrc).toBe("/assets/foo.png");
    expect(image.dataset.m2hLazyState).toBe("pending");
    expect(image.getAttribute("aria-busy")).toBe("true");
    // The alt is the image's accessible name and the tooltip's source — it
    // must survive the rewrite untouched.
    expect(image.getAttribute("alt")).toBe("foo");
  });

  it("parks srcset so no real candidate stays in the pending DOM", () => {
    const root = mount('<img src="a.png" srcset="a.png 1x, a@2x.png 2x">');
    const image = imageIn(root);

    expect(image.getAttribute("srcset")).toBeNull();
    expect(image.dataset.m2hOriginalSrcset).toBe("a.png 1x, a@2x.png 2x");
    expect(image.getAttribute("src")).toBe(IMAGE_LOADING_SRC);
  });

  it("parks a picture's source candidates the same way", () => {
    const root = mount(
      `<picture>
        <source media="(prefers-color-scheme: dark)" srcset="dark.png">
        <img src="light.png" alt="theme">
      </picture>`,
    );
    const source = root.querySelector("source");
    const image = imageIn(root);

    // The request-issuing attribute moves; media selection metadata stays.
    expect(source?.getAttribute("srcset")).toBeNull();
    expect(source?.dataset.m2hOriginalSrcset).toBe("dark.png");
    expect(source?.getAttribute("media")).toBe("(prefers-color-scheme: dark)");
    expect(image.dataset.m2hOriginalSrc).toBe("light.png");
    expect(image.getAttribute("src")).toBe(IMAGE_LOADING_SRC);
  });

  it("leaves images that request nothing untouched", () => {
    const root = mount('<p><img alt="decorative"></p>');
    const image = imageIn(root);

    // Rewriting a sourceless image would start the very request lazy loading
    // exists to avoid: no placeholder, no state, no parked attributes.
    expect(image.getAttribute("src")).toBeNull();
    expect(image.dataset.m2hLazyState).toBeUndefined();
    expect(image.dataset.m2hOriginalSrc).toBeUndefined();
    expect(image.getAttribute("aria-busy")).toBeNull();
  });
});

describe("observeLazyImages", () => {
  beforeEach(() => {
    FakeIntersectionObserver.instances = [];
    vi.stubGlobal("IntersectionObserver", FakeIntersectionObserver);
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("observes every pending image with the documented load margin", () => {
    const root = mount(
      '<p><img src="/a.png" alt="a"></p><p><img src="/b.png" alt="b"></p>',
    );
    const controller = observeLazyImages(root);
    const observer = FakeIntersectionObserver.instances.at(-1);

    expect(observer?.observed).toHaveLength(2);
    expect(observer?.options?.rootMargin).toBe("512px 0px");
    expect(observer?.options?.threshold).toBe(0);
    controller.disconnect();
  });

  it("restores the real sources when an image approaches the viewport", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    observeLazyImages(root);
    const observer = FakeIntersectionObserver.instances.at(-1);

    observer?.intersect([image]);

    expect(image.getAttribute("src")).toBe("/assets/foo.png");
    expect(image.dataset.m2hLazyState).toBe("loading");
    expect(image.getAttribute("aria-busy")).toBe("true");
    // The parked copies stay while the request is in flight: with the real
    // src already on the element, a stale currentSrc can still point at the
    // placeholder, so the parked attribute is the only stable source of
    // truth for consumers snapshotting mid-flight (the Lightbox).
    expect(image.dataset.m2hOriginalSrc).toBe("/assets/foo.png");
  });

  it("restores a picture's sources in candidate-then-img order", () => {
    const root = mount(
      `<picture>
        <source media="(prefers-color-scheme: dark)" srcset="dark.png">
        <img src="light.png" alt="theme">
      </picture>`,
    );
    const source = root.querySelector("source");
    const image = imageIn(root);
    observeLazyImages(root);
    const observer = FakeIntersectionObserver.instances.at(-1);

    observer?.intersect([image]);

    // Both candidates are back before the load event can pick between them.
    expect(source?.getAttribute("srcset")).toBe("dark.png");
    expect(image.getAttribute("src")).toBe("light.png");
    // The parked copies stay until settle, like a plain image's.
    expect(source?.dataset.m2hOriginalSrcset).toBe("dark.png");
    expect(image.dataset.m2hOriginalSrc).toBe("light.png");
  });

  it("settles a load as loaded and stops observing the image", () => {
    const settled: Array<[HTMLImageElement, boolean]> = [];
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    observeLazyImages(root, {
      onImageSettled: (settledImage, loaded) => {
        settled.push([settledImage, loaded]);
      },
    });
    const observer = FakeIntersectionObserver.instances.at(-1);

    observer?.intersect([image]);
    image.dispatchEvent(new Event("load"));

    expect(image.dataset.m2hLazyState).toBe("loaded");
    expect(image.getAttribute("aria-busy")).toBeNull();
    expect(observer?.observed).not.toContain(image);
    expect(settled).toEqual([[image, true]]);
    // Settled as loaded: the live attributes carry the real sources, so the
    // parked copies are cleaned up instead of shadowing them.
    expect(image.dataset.m2hOriginalSrc).toBeUndefined();
    expect(image.dataset.m2hOriginalSrcset).toBeUndefined();
  });

  it("settles an error as failed without touching the presentation", () => {
    const settled: Array<[HTMLImageElement, boolean]> = [];
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    observeLazyImages(root, {
      onImageSettled: (settledImage, loaded) => {
        settled.push([settledImage, loaded]);
      },
    });
    const observer = FakeIntersectionObserver.instances.at(-1);

    observer?.intersect([image]);
    image.dispatchEvent(new Event("error"));

    expect(image.dataset.m2hLazyState).toBe("failed");
    expect(image.getAttribute("aria-busy")).toBeNull();
    expect(settled).toEqual([[image, false]]);
    // The failed machine keeps the parked source: the top warning reports
    // the author's URL from it after the failure placeholder swapped in.
    expect(image.dataset.m2hOriginalSrc).toBe("/assets/foo.png");
  });

  it("ignores a load delivered while the image is still pending", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    observeLazyImages(root);

    // The placeholder's own load event arrives while the image is pending —
    // it must not settle the machine before the real sources were restored.
    image.dispatchEvent(new Event("load"));

    expect(image.dataset.m2hLazyState).toBe("pending");
    expect(image.getAttribute("src")).toBe(IMAGE_LOADING_SRC);
  });

  it("absorbs a second intersection round for a settled image", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    observeLazyImages(root);
    const observer = FakeIntersectionObserver.instances.at(-1);

    observer?.intersect([image]);
    image.dispatchEvent(new Event("load"));
    // A stale delivery or an observer that never unobserved must not restart
    // the machine — the state guard absorbs it.
    observer?.intersect([image]);

    expect(image.dataset.m2hLazyState).toBe("loaded");
    expect(image.getAttribute("src")).toBe("/assets/foo.png");
  });

  it("keeps one machine when a repeated pass schedules the same images", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    // A repeated observation pass (a second enhancement sweep) schedules the
    // still-pending image again; whichever round fires first loads it, and
    // the other's delivery must be absorbed instead of re-restoring.
    const first = observeLazyImages(root);
    const second = observeLazyImages(root);

    FakeIntersectionObserver.instances[0]?.intersect([image]);
    FakeIntersectionObserver.instances[1]?.intersect([image]);
    image.dispatchEvent(new Event("load"));

    expect(image.dataset.m2hLazyState).toBe("loaded");
    expect(image.getAttribute("src")).toBe("/assets/foo.png");
    expect(image.dataset.m2hOriginalSrc).toBeUndefined();
    first.disconnect();
    second.disconnect();
  });

  it("disconnect stops observation and leaves the images pending", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    const controller = observeLazyImages(root);
    const observer = FakeIntersectionObserver.instances.at(-1);

    controller.disconnect();
    // Deliveries after the disconnect are the observer's problem, but the
    // module must not have restored anything on its own.
    expect(observer?.disconnected).toBe(true);
    expect(image.dataset.m2hLazyState).toBe("pending");
    expect(image.getAttribute("src")).toBe(IMAGE_LOADING_SRC);
  });

  it("skips a slot the failure pipeline already collapsed", () => {
    const root = mount('<p><img src="/assets/foo.png" alt="foo"></p>');
    const image = imageIn(root);
    // What replaceImageWithFallback leaves behind: the failed placeholder is
    // showing and the element is marked as the fallback.
    image.dataset.m2hFallback = "true";
    image.setAttribute("src", "/ui/image-load-failed.svg");

    observeLazyImages(root);
    const observer = FakeIntersectionObserver.instances.at(-1);
    observer?.intersect([image]);

    // Restoring would overwrite the failure placeholder with the source that
    // already failed to load.
    expect(image.dataset.m2hLazyState).toBe("pending");
    expect(image.getAttribute("src")).toBe("/ui/image-load-failed.svg");
  });

  it("eagerly starts loading without IntersectionObserver", () => {
    vi.unstubAllGlobals();
    // jsdom has no IntersectionObserver: the environment check must take the
    // fallback and restore every parked source right away — eager loading,
    // not eager settling. The machine still follows the real load/error
    // events, so the presentation withheld for a loading image (aria-busy,
    // tooltip, Lightbox trigger) stays withheld until the picture is there.
    expect("IntersectionObserver" in window).toBe(false);

    const settled: Array<[HTMLImageElement, boolean]> = [];
    const root = mount(
      '<p><img src="/assets/foo.png" alt="foo"></p><p><img alt="bare"></p>',
    );
    const image = imageIn(root);
    observeLazyImages(root, {
      onImageSettled: (settledImage, loaded) => {
        settled.push([settledImage, loaded]);
      },
    });

    expect(image.getAttribute("src")).toBe("/assets/foo.png");
    expect(image.dataset.m2hLazyState).toBe("loading");
    expect(image.getAttribute("aria-busy")).toBe("true");
    expect(settled).toEqual([]);

    image.dispatchEvent(new Event("load"));
    expect(image.dataset.m2hLazyState).toBe("loaded");
    expect(image.getAttribute("aria-busy")).toBeNull();
    expect(settled).toEqual([[image, true]]);
  });
});
