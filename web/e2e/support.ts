import { expect, type Page } from "@playwright/test";

// Helpers shared by the real-browser suites (layout.spec.ts and the WebKit
// mobile scroll smoke). Playwright registers tests only in *.spec.ts files,
// so this module stays a plain library.

// Wait until the document body has painted, so geometry assertions never race
// the client-rendered article.
export async function waitForBody(page: Page, path: string) {
  await page.goto(path);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body p, .markdown-body h2") !== null,
  );
}

// One geometry snapshot of the sidebar's single scroll container — the
// [data-slot="scroll-area-viewport"] box, whether it is the desktop Base UI
// ScrollArea viewport or the mobile native scroller — so isolation and
// normalization assertions all read the same source of truth.
export async function readSidebarGeometry(page: Page) {
  return page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (!(viewport instanceof HTMLElement)) {
      throw new Error("tree viewport was not rendered");
    }
    const rect = viewport.getBoundingClientRect();
    return {
      scrollTop: viewport.scrollTop,
      scrollLeft: viewport.scrollLeft,
      scrollHeight: viewport.scrollHeight,
      clientHeight: viewport.clientHeight,
      scrollWidth: viewport.scrollWidth,
      clientWidth: viewport.clientWidth,
      windowScrollY: window.scrollY,
      x: rect.x,
      y: rect.y,
      width: rect.width,
      height: rect.height,
    };
  });
}

// Cold-start opening of the mobile sidebar sheet, shared by the first-touch
// regressions in layout.spec.ts and the WebKit scroll smoke. Real phones used
// to leave the very first swipe dead until some tree interaction rebuilt the
// scrolling layer, and the earlier regression could not see that because it
// preheated the viewport itself: a bottom-of-tree document made the
// active-file reveal scroll, then the test wrote scrollTop = 0 on top. Both
// are forbidden here — the document sits at the top of the tree so the reveal
// has nothing to correct, and nothing may touch the scroll position
// afterwards.
export async function openColdMobileSidebar(page: Page) {
  await page.setViewportSize({ width: 375, height: 720 });
  // note-01 renders directly below its expanded `tree` directory row, so the
  // active-file reveal keeps the viewport at scrollTop 0; a bottom-of-tree
  // document (note-24) would scroll it before the first swipe.
  await waitForBody(page, "/doc/tree/note-01.md");

  await page.getByRole("button", { name: "切换文件导航" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();

  // The mobile scroller stays the tree's only scroll container — the native
  // overflow-y: auto box (.tree-native-scroll) — it really overflows, and the
  // mobile SidebarContent clips nothing.
  const structure = await page.evaluate(() => {
    const content = document.querySelector('[data-slot="sidebar-content"]');
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (!(content instanceof HTMLElement)) {
      throw new Error("sidebar content was not rendered");
    }
    if (!(viewport instanceof HTMLElement)) {
      throw new Error("tree viewport was not rendered");
    }
    return {
      contentOverflow: getComputedStyle(content).overflow,
      viewportOverflowY: getComputedStyle(viewport).overflowY,
      overflowing: viewport.scrollHeight > viewport.clientHeight,
    };
  });
  expect(structure.contentOverflow).toBe("visible");
  expect(structure.viewportOverflowY).toBe("auto");
  expect(structure.overflowing).toBe(true);

  // The cold-start invariant: no reveal, no normalization, no restore has
  // moved the tree viewport, and the reader window behind the modal sheet is
  // still at the top.
  const before = await readSidebarGeometry(page);
  expect(before.scrollTop).toBe(0);
  expect(before.windowScrollY).toBe(0);
}
