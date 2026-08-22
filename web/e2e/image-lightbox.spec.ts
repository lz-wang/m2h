import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for the image Lightbox. The behaviors that matter
// here are exactly the ones jsdom cannot compute: the modal dialog's scroll
// lock, the portaled overlay's layout, and the two hard invariants of the
// feature — through every lightbox interaction (open, next/previous, zoom,
// rotate, pan, close) the window's scrollY and the URL hash must not move, and
// the reading position must be exactly where it was left.

const documentPath = "/doc/image-lightbox.md";

// Wait until the article has painted and the lightbox triggers are injected,
// then expose the toolbar/counter helpers every test below shares.
async function openDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(documentPath);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body img") !== null &&
      document.querySelectorAll(".m2h-image-lightbox-trigger").length >= 3,
  );
}

// Bring the first trigger into view explicitly, then let the heading spy
// settle so the recorded hash is the stable pre-lightbox state. Returning the
// invariants this way means no later Playwright auto-scroll can pollute them.
async function captureInvariants(page: Page) {
  await page
    .locator(".m2h-image-lightbox-trigger")
    .first()
    .scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  return {
    scrollY: await page.evaluate(() => window.scrollY),
    hash: await page.evaluate(() => location.hash),
  };
}

async function expectInvariantsUnchanged(
  page: Page,
  before: { scrollY: number; hash: string },
) {
  const afterScrollY = await page.evaluate(() => window.scrollY);
  const afterHash = await page.evaluate(() => location.hash);
  expect(Math.abs(afterScrollY - before.scrollY)).toBeLessThanOrEqual(1);
  expect(afterHash).toBe(before.hash);
}

async function openLightbox(page: Page, triggerIndex = 0) {
  await page.locator(".m2h-image-lightbox-trigger").nth(triggerIndex).click();
  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  return popup;
}

// The pan component of the current transform, parsed out of the inline style.
async function imagePan(page: Page): Promise<{ x: number; y: number }> {
  return page.evaluate(() => {
    const image = document.querySelector<HTMLImageElement>(
      ".image-lightbox-image",
    );
    if (image === null) {
      throw new Error("lightbox image was not rendered");
    }
    const match = image.style.transform.match(
      /translate3d\((-?[\d.]+)px, (-?[\d.]+)px, 0px\)/,
    );
    if (match === null) {
      throw new Error(`unexpected transform: ${image.style.transform}`);
    }
    return { x: Number(match[1]), y: Number(match[2]) };
  });
}

// Wait until the lightbox image has been laid out at its fitted size: the
// geometry the rotate fit and the pan clamp consume is reported by the
// ResizeObserver only once the element has a box.
async function waitForFittedImage(
  page: Page,
  min: number,
): Promise<{ x: number; y: number; width: number; height: number }> {
  const image = page.locator(".image-lightbox-image");
  await expect
    .poll(async () => (await image.boundingBox())?.height ?? 0, {
      timeout: 5_000,
    })
    .toBeGreaterThanOrEqual(min);
  const box = await image.boundingBox();
  if (box === null) {
    throw new Error("lightbox image was not rendered");
  }
  return box;
}

test("keeps scrollY and hash invariant across a full lightbox session", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  const displayed = page.locator(".image-lightbox-image");
  const counter = page.locator(
    '.image-lightbox-counter > span[aria-hidden="true"]',
  );
  await expect(counter).toHaveText("1 / 3");
  // The fixtures are three different files, so the counter and the actually
  // displayed image must switch together — an index-only regression (the
  // stale-index table-sort bug) shows the wrong file here.
  await expect(displayed).toHaveAttribute("src", /landscape\.png$/);

  // Navigate, rotate, zoom — the whole toolbar round.
  await page.getByRole("button", { name: "下一张图片" }).click();
  await expect(counter).toHaveText("2 / 3");
  await expect(displayed).toHaveAttribute("src", /portrait\.png$/);
  // The portrait fixture (480×1200) fits the stage height-first.
  const portraitBox = await waitForFittedImage(page, 700);
  expect(portraitBox.height).toBeGreaterThan(portraitBox.width);
  await page.getByRole("button", { name: "下一张图片" }).click();
  await expect(counter).toHaveText("3 / 3");
  await expect(displayed).toHaveAttribute("src", /square\.png$/);
  await page.getByRole("button", { name: "顺时针旋转" }).click();
  await page.getByRole("button", { name: "放大图片" }).click();

  // The transform carries the rotated, zoomed state on the compositor.
  const transform = await page.evaluate(() => {
    const image = document.querySelector<HTMLImageElement>(
      ".image-lightbox-image",
    );
    if (image === null) {
      throw new Error("lightbox image was not rendered");
    }
    return image.style.transform;
  });
  expect(transform).toContain("rotate(90deg)");
  expect(transform).toContain("scale(");
  expect(transform).not.toContain("scale(1)");

  // Close through the X button; the lightbox is gone and nothing moved.
  await page.getByRole("button", { name: "关闭图片预览" }).click();
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("closes through Escape without moving the document", async ({ page }) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  await page.getByRole("button", { name: "下一张图片" }).click();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("2 / 3");

  await page.keyboard.press("Escape");
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("closes through a blank-area press without moving the document", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  await page.getByRole("button", { name: "下一张图片" }).click();

  // A press on the overlay's top-left corner: empty scrim, away from both the
  // centered image and the top-right close button.
  await page.mouse.click(12, 12);
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("does not close on presses over the image or the toolbar", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  // Press the toolbar (its zoom button), then the image itself.
  await page.getByRole("button", { name: "放大图片" }).click();
  await page.locator(".image-lightbox-image").click();
  await expect(page.locator(".image-lightbox")).toBeVisible();
  await expectInvariantsUnchanged(page, before);
});

test("disables previous on the first image and next on the last", async ({
  page,
}) => {
  await openDocument(page);
  await captureInvariants(page);

  await openLightbox(page, 0);
  const previous = page.getByRole("button", { name: "上一张图片" });
  await expect(previous).toBeDisabled();

  const next = page.getByRole("button", { name: "下一张图片" });
  await next.click();
  await next.click();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("3 / 3");
  await expect(next).toBeDisabled();
  await expect(previous).toBeEnabled();
});

test("keeps a rotated landscape image clear of the bottom toolbar", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  // The first image is the 1200×600 landscape fixture: tall enough that a
  // quarter turn produces a portrait taller than the toolbar-free area, so a
  // fit computed against the whole viewport would slide under the toolbar.
  await openLightbox(page, 0);
  const toolbar = page.locator(".image-lightbox-toolbar");

  // Unrotated fit: the image already sits inside the stage.
  let imageBox = await waitForFittedImage(page, 599);
  let toolbarBox = await toolbar.boundingBox();
  expect(toolbarBox).not.toBeNull();
  expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(toolbarBox.y);

  // A quarter turn swaps the visual axes; the re-fit must land inside the
  // stage (the popup minus the toolbar reserve), never under the toolbar.
  await page.getByRole("button", { name: "顺时针旋转" }).click();
  imageBox = await waitForFittedImage(page, 700);
  toolbarBox = await toolbar.boundingBox();
  expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(toolbarBox.y + 1);

  await expectInvariantsUnchanged(page, before);
});

test("clamps pointer pans to the fitted stage after zooming", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  const start = await waitForFittedImage(page, 599);

  // Zoom to the 5x cap: the 1200×600 landscape at 1280×900 sits at scale 5,
  // so the drag bounds are maxPanX = (1200·5 − 1248)/2 = 2376 and maxPanY =
  // (600·5 − 788)/2 = 1106.
  const zoomIn = page.getByRole("button", { name: "放大图片" });
  for (let click = 0; click < 8; click += 1) {
    await zoomIn.click();
  }
  await expect(zoomIn).toBeDisabled();

  // Drag far past every edge; the pan must stop at the fitted bound instead
  // of following the pointer.
  await page.mouse.move(start.x + start.width / 2, start.y + start.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    start.x + start.width / 2 - 5000,
    start.y + start.height / 2 - 5000,
    { steps: 10 },
  );
  await page.mouse.up();

  const pan = await imagePan(page);
  expect(pan.x).toBeGreaterThanOrEqual(-2377);
  expect(pan.x).toBeLessThanOrEqual(-2375);
  expect(pan.y).toBeGreaterThanOrEqual(-1107);
  expect(pan.y).toBeLessThanOrEqual(-1105);

  await expectInvariantsUnchanged(page, before);
});

test("keeps the linked image's anchor while its trigger opens the lightbox", async ({
  page,
}) => {
  await openDocument(page);

  // Pressing the linked image itself follows the anchor: the fragment moves
  // and no lightbox appears.
  await page.locator(".m2h-image-frame a img").scrollIntoViewIfNeeded();
  await page.locator(".m2h-image-frame a img").click();
  await page.waitForFunction(() => decodeURIComponent(location.hash) !== "");
  expect(decodeURIComponent(await page.evaluate(() => location.hash))).toBe(
    "#目标章节",
  );
  expect(await page.locator(".image-lightbox").count()).toBe(0);

  // The magnifier on the same frame opens the lightbox instead.
  await page
    .locator(".m2h-image-frame:has(> a) .m2h-image-lightbox-trigger")
    .click();
  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("2 / 3");
});
