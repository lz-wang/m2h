import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for lazy document images. The one thing that
// matters here is invisible to jsdom and to DOM-only assertions: whether the
// browser actually issued (or withheld) the network request for a far
// below-the-fold image. The fixture therefore parks its image well past the
// viewport plus the 512px load margin, and the test listens to the requests
// the page really makes.

const documentPath = "/doc/image-lazy-loading.md";
const realImagePattern = /\/assets\/images\/lazy-image\.png/;
const placeholderSrc = "/ui/image-loading.svg";

// Listen for the real image's requests and open the document. Returns the
// push array the test asserts against.
function trackRealImageRequests(page: Page): string[] {
  const requests: string[] = [];
  page.on("request", (request) => {
    if (realImagePattern.test(request.url())) {
      requests.push(request.url());
    }
  });
  return requests;
}

test("keeps a below-the-fold image unloaded until it approaches the viewport", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  const requests = trackRealImageRequests(page);

  await page.goto(documentPath);
  await page.waitForFunction(
    () => document.querySelector(".markdown-body img") !== null,
  );

  // Still parked: the placeholder is showing, the machine is pending, and
  // the browser never asked for the real bytes.
  const image = page.locator(".markdown-body img");
  await expect(image).toHaveAttribute("src", placeholderSrc);
  await expect(image).toHaveAttribute("data-m2h-lazy-state", "pending");
  await expect(image).toHaveAttribute(
    "data-m2h-original-src",
    "/assets/images/lazy-image.png",
  );
  await expect(image).toHaveAttribute("aria-busy", "true");
  expect(requests).toEqual([]);

  // Scrolling the image into view hands it its real source back and the
  // load settles the machine.
  await image.scrollIntoViewIfNeeded();
  await expect(image).toHaveAttribute("src", "/assets/images/lazy-image.png");
  await expect(image).toHaveAttribute("data-m2h-lazy-state", "loaded");
  await expect(image).not.toHaveAttribute("aria-busy");
  expect(requests.length).toBeGreaterThan(0);
});

test("withholds the Lightbox and tooltip while the placeholder shows", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await page.goto(documentPath);
  await page.waitForFunction(
    () => document.querySelector(".markdown-body img") !== null,
  );

  const frame = page.locator(".m2h-image-frame");
  const image = page.locator(".markdown-body img");
  await expect(image).toHaveAttribute("data-m2h-lazy-state", "pending");

  // A placeholder is nothing to magnify: the trigger hides, so a pending
  // image cannot be opened. (The item marker stays — a loaded neighbor's
  // next/previous still addresses it, by its parked real source.) No hover
  // here: hovering would scroll the image into the load margin and load it
  // mid-assertion.
  const trigger = frame.locator(".m2h-lightbox-trigger");
  await expect(trigger).toBeHidden();

  // The metadata tooltip would only be able to describe the placeholder's
  // own intrinsic size — the stylesheet keeps it out of layout entirely.
  const tooltip = frame.locator(".m2h-image-name-tooltip");
  await expect(tooltip).toHaveCSS("display", "none");

  await image.scrollIntoViewIfNeeded();
  await expect(image).toHaveAttribute("data-m2h-lazy-state", "loaded");

  // Loaded: the presentation follows — trigger available, tooltip back in
  // layout.
  await expect(trigger).toBeVisible();
  await expect(tooltip).toHaveCSS("display", "flex");
});
