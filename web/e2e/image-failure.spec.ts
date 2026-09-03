import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for the failed-image placeholder. The 404 round
// trip (the browser actually requests the missing asset, the error event
// arrives after enhancement) is invisible to jsdom, which never loads
// resources — only a real server + browser pair exercises the swap.

const documentPath = "/doc/image-failure.md";

async function openDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(documentPath);
  await page.waitForSelector(".m2h-image-failed");
}

test("collapses the broken image into the shared placeholder", async ({
  page,
}) => {
  await openDocument(page);

  const frame = page.locator(".m2h-image-failed");
  await expect(frame.locator("img")).toHaveAttribute(
    "src",
    "/image-load-failed.svg",
  );
  // The placeholder is what the accessibility tree reads: the original alt
  // described a picture that never showed up.
  await expect(frame.locator("img")).toHaveAttribute("alt", "图片加载失败");
  // The failed source stays on the element for the top warning to report
  // (link rewriting resolved the document-relative reference to its
  // root-relative /assets address before the browser fetched it).
  await expect(frame.locator("img")).toHaveAttribute(
    "data-m2h-original-src",
    "/assets/images/does-not-exist.png",
  );

  // No name/size tooltip on a placeholder, and no magnifier either: the
  // trigger hides (display: none), so it cannot be hovered into view.
  await expect(frame.locator(".m2h-image-name-tooltip")).toHaveCount(0);
  const trigger = frame.locator(".m2h-lightbox-trigger");
  await expect(trigger).toBeHidden();

  await frame.hover();
  await expect(trigger).toBeHidden();
});

test("keeps the failed image out of the lightbox and reports the original source", async ({
  page,
}) => {
  await openDocument(page);

  // Poking the placeholder must not open a lightbox over a placeholder.
  await page.locator(".m2h-image-failed").click();
  await page.locator(".m2h-image-failed img").click({ force: true });
  await expect(page.locator(".image-lightbox")).toHaveCount(0);

  // The top warning names the asset that actually failed, never the
  // placeholder that replaced it.
  await expect(page.locator(".asset-warning")).toContainText(
    "附件加载失败：/assets/images/does-not-exist.png",
  );
});
