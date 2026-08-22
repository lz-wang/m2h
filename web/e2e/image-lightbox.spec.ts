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

test("keeps scrollY and hash invariant across a full lightbox session", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("1 / 3");

  // Navigate, rotate, zoom — the whole toolbar round.
  await page.getByRole("button", { name: "下一张图片" }).click();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("2 / 3");
  await page.getByRole("button", { name: "下一张图片" }).click();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("3 / 3");
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
  const image = page.locator(".image-lightbox-image");
  const toolbar = page.locator(".image-lightbox-toolbar");

  // The rotate fit consumes geometry the ResizeObserver reports once the
  // lightbox image is laid out; wait for the nonzero fitted box so the
  // assertions below measure the real fit, not the pre-measure fallback.
  const fittedHeight = async () => (await image.boundingBox())?.height ?? 0;
  await expect
    .poll(fittedHeight, { timeout: 5_000 })
    .toBeGreaterThanOrEqual(599);

  // Unrotated fit: the image already sits inside the stage.
  let imageBox = await image.boundingBox();
  let toolbarBox = await toolbar.boundingBox();
  expect(imageBox).not.toBeNull();
  expect(toolbarBox).not.toBeNull();
  expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(toolbarBox.y);

  // A quarter turn swaps the visual axes; the re-fit must land inside the
  // stage (the popup minus the toolbar reserve), never under the toolbar.
  await page.getByRole("button", { name: "顺时针旋转" }).click();
  await expect
    .poll(fittedHeight, { timeout: 5_000 })
    .toBeGreaterThanOrEqual(700);
  imageBox = await image.boundingBox();
  toolbarBox = await toolbar.boundingBox();
  expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(toolbarBox.y + 1);

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
