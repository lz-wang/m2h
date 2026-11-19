import { expect, test } from "@playwright/test";
import {
  firstTouchSwipeFromRow,
  openColdMobileSidebar,
  waitForBody,
} from "./support";

// Coverage on true mobile device profiles (hasTouch + isMobile emulation),
// not just a narrow desktop viewport: the sidebar sheet's focus and first
// swipe contracts must hold for real touch opens. The focus test runs on
// every engine matched here; the touch-drag regressions drive CDP, which
// exists only in Chromium, so they skip elsewhere.

test("opens the mobile sidebar focused on the sheet, not the filter input", async ({
  page,
}) => {
  // A real touch tap on the toolbar toggle: the sheet is still opened
  // through app state (no Dialog.Trigger), so Base UI records no open
  // interaction type — the sheet itself must own the initial focus instead
  // of letting the default fall to the filter input, whose keyboard path
  // derails touch scrolling on phones.
  await waitForBody(page, "/doc/tree/note-01.md");
  await page.getByRole("button", { name: "切换文件导航" }).tap();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  await expect(
    page.getByRole("searchbox", { name: "按文件名或路径筛选文件" }),
  ).not.toBeFocused();
  await expect
    .poll(() =>
      dialog.evaluate((element) => document.activeElement === element),
    )
    .toBe(true);
});

test("opens home and closes the mobile sidebar on tap", async ({ page }) => {
  await openColdMobileSidebar(page);
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("button", { name: "文档首页" }).tap();

  await expect(dialog).toBeHidden();
  await expect(page.getByRole("article", { name: "文档首页" })).toBeVisible();
});

test("scrolls the mobile sidebar on the first untouched swipe from a directory row", async ({
  page,
  browserName,
}) => {
  test.skip(browserName !== "chromium", "CDP touch is Chromium-only");
  await openColdMobileSidebar(page);
  await firstTouchSwipeFromRow(
    page,
    page.locator('[data-tree-directory="true"][data-tree-path="a"]'),
  );
});

test("scrolls the mobile sidebar on the first untouched swipe from a file row", async ({
  page,
  browserName,
}) => {
  test.skip(browserName !== "chromium", "CDP touch is Chromium-only");
  await openColdMobileSidebar(page);
  await firstTouchSwipeFromRow(page, page.locator('[aria-current="page"]'));
});
