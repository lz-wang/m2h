import { expect, test } from "@playwright/test";
import {
  openColdMobileSidebar,
  readSidebarGeometry,
  readTocGeometry,
  waitForBody,
} from "./support";

// WebKit coverage for the mobile scroll contracts. The dead-first-swipe
// reports come from WebKit phones while Chromium stayed green, so these smoke
// tests pin the same contracts on Playwright's WebKit engine: the cold open
// exposes an overflowing scroll box at scrollTop 0, a real trusted input
// gesture moves it, and the reader window behind the modal sheet never moves.
//
// Playwright's WebKit cannot synthesize a touch drag (the CDP touch sequence
// used by layout.spec.ts is Chromium-only), so the gesture input here is a
// mouse wheel — still engine-level scrolling of the same box; the touch-drag
// regressions keep running under the Chromium project.

// Move the pointer onto the row's real center and wheel down: the nearest
// scrollable ancestor under the pointer must take the gesture.
async function wheelDownFromRow(
  page: import("@playwright/test").Page,
  row: import("@playwright/test").Locator,
) {
  const box = await row.boundingBox();
  if (box === null) {
    throw new Error("wheel start row was not rendered");
  }
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.wheel(0, 400);

  await expect
    .poll(async () => (await readSidebarGeometry(page)).scrollTop)
    .toBeGreaterThan(0);
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
}

test("scrolls the mobile sidebar tree from the directory row on WebKit", async ({
  page,
}) => {
  // A plain directory row: no tooltip or context-menu wrapper around it, so
  // the gesture targets the native scroll box itself.
  await openColdMobileSidebar(page);
  await wheelDownFromRow(
    page,
    page.locator('[data-tree-directory="true"][data-tree-path="a"]'),
  );
});

test("scrolls the mobile sidebar tree from the file row on WebKit", async ({
  page,
}) => {
  // The active file row: a button wrapped in a tooltip trigger and a
  // context-menu trigger, so the gesture passes through the interactive
  // file-row machinery before reaching the scroll box.
  await openColdMobileSidebar(page);
  await wheelDownFromRow(page, page.locator('[aria-current="page"]'));
});

test("scrolls the mobile TOC sheet on WebKit from the cold open", async ({
  page,
}) => {
  // The TOC sheet keeps the Base UI ScrollArea on every viewport, so this
  // pins the sibling contract the sidebar's native scroller converged to:
  // on a cold open with a long outline the sheet really overflows and takes
  // the gesture, without chaining into the reader window.
  await page.setViewportSize({ width: 375, height: 720 });
  // toc-scroll.md carries far more headings than one screen can hold.
  await waitForBody(page, "/doc/toc-scroll.md");

  await page.getByRole("button", { name: "打开文档目录" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  const readToc = () => readTocGeometry(page);

  const before = await readToc();
  expect(before.scrollTop).toBe(0);
  expect(before.scrollHeight).toBeGreaterThan(before.clientHeight);
  expect(before.windowScrollY).toBe(0);

  const box = await dialog.boundingBox();
  if (box === null) {
    throw new Error("TOC sheet was not rendered");
  }
  await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
  await page.mouse.wheel(0, 400);

  await expect.poll(async () => (await readToc()).scrollTop).toBeGreaterThan(0);
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
});
