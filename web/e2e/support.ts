import { expect, type Locator, type Page } from "@playwright/test";

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
// regressions in mobile-sidebar.spec.ts under the mobile device profiles
// (hasTouch + isMobile) — the open is a real touch tap, matching how phones
// open the sheet. Real phones used to leave the very first swipe dead until
// some tree interaction rebuilt the scrolling layer, and the earlier
// regression could not see that because it preheated the viewport itself: a
// bottom-of-tree document made the active-file reveal scroll, then the test
// wrote scrollTop = 0 on top. Both are forbidden here — the document sits at
// the top of the tree so the reveal has nothing to correct, and nothing may
// touch the scroll position afterwards.
export async function openColdMobileSidebar(page: Page) {
  // note-01 renders directly below its expanded `tree` directory row, so the
  // active-file reveal keeps the viewport at scrollTop 0; a bottom-of-tree
  // document (note-24) would scroll it before the first swipe.
  await waitForBody(page, "/doc/tree/note-01.md");

  await page.getByRole("button", { name: "切换文件导航" }).tap();
  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();

  // The ScrollArea viewport stays the tree's only scroll container (the Base
  // UI ScrollArea, same as desktop), it really overflows, and the mobile
  // SidebarContent clips nothing.
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
  expect(structure.viewportOverflowY).toBe("scroll");
  expect(structure.overflowing).toBe(true);

  // The sheet must not park focus in the filter input: a controlled open
  // records no openMethod in Base UI, so its default initial focus falls to
  // the first tabbable element — the filter input — whose keyboard path
  // (virtual keyboard, visual viewport) was the standing suspect behind the
  // dead first swipe on phones. Touch and pointer opens must focus the sheet
  // popup itself; keyboard keeps the default first-field behavior.
  const search = page.getByRole("searchbox", { name: "筛选文件" });
  await expect(search).not.toBeFocused();
  await expect
    .poll(() =>
      dialog.evaluate((element) => document.activeElement === element),
    )
    .toBe(true);

  // The cold-start invariant: no reveal, no normalization, no restore has
  // moved the tree viewport, and the reader window behind the modal sheet is
  // still at the top.
  const before = await readSidebarGeometry(page);
  expect(before.scrollTop).toBe(0);
  expect(before.windowScrollY).toBe(0);
}

// One genuine touch gesture and nothing else: touchStart on the row's real
// center, five upward touchMoves, touchEnd. Between opening the sheet and this
// swipe there must be no tree click, no focus, no scrollTop write, no wheel,
// no scrollIntoView, no expand/collapse — any of those would rebuild the
// scrolling layer and turn the assertion into a preheated false negative.
// CDP touch is Chromium-only, so callers outside Chromium must skip.
export async function firstTouchSwipeFromRow(page: Page, row: Locator) {
  const box = await row.boundingBox();
  if (box === null) {
    throw new Error("swipe start row was not rendered");
  }
  const startX = box.x + box.width / 2;
  const startY = box.y + box.height / 2;

  const session = await page.context().newCDPSession(page);
  await session.send("Input.dispatchTouchEvent", {
    type: "touchStart",
    touchPoints: [{ x: startX, y: startY }],
  });
  for (const offset of [40, 80, 120, 160, 200]) {
    await session.send("Input.dispatchTouchEvent", {
      type: "touchMove",
      touchPoints: [{ x: startX, y: Math.max(startY - offset, 8) }],
    });
  }
  await session.send("Input.dispatchTouchEvent", {
    type: "touchEnd",
    touchPoints: [],
  });

  await expect
    .poll(async () => (await readSidebarGeometry(page)).scrollTop)
    .toBeGreaterThan(0);
  expect(await page.evaluate(() => window.scrollY)).toBe(0);
}
