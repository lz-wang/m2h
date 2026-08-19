import { expect, test } from "@playwright/test";

// Real-browser layout regressions for the reader shell. jsdom computes no
// geometry, so horizontal centering, the narrow-screen outline sheet, and the
// sidebar reveal of the active file can only be locked down with a genuine
// layout engine — the same reasoning as scroll-restoration.spec.ts.

// Wait until the document body has painted, so geometry assertions never race
// the client-rendered article.
async function waitForBody(
  page: import("@playwright/test").Page,
  path: string,
) {
  await page.goto(path);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body p, .markdown-body h2") !== null,
  );
}

// Two animation frames: any scroll a wheel dispatch could have produced is
// applied before the next frame paints, so positions read afterwards reflect
// the settled state — without an arbitrary timeout.
async function waitForScrollSettle(page: import("@playwright/test").Page) {
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
}

// One geometry snapshot of the sidebar's single scroll container (the Base UI
// ScrollArea viewport), so isolation and normalization assertions all read
// the same source of truth.
async function readSidebarGeometry(page: import("@playwright/test").Page) {
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

async function setSidebarScrollTop(
  page: import("@playwright/test").Page,
  where: "top" | "max",
) {
  await page.evaluate((target) => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport instanceof HTMLElement) {
      viewport.scrollTop =
        target === "top" ? 0 : viewport.scrollHeight - viewport.clientHeight;
    }
  }, where);
}

// Land on the nested fixture with the tree viewport already scrolled deep
// past both sticky ancestors — the reload + reveal flow from the tests below,
// factored out for the collapse regressions.
async function openRevealedNestedFile(page: import("@playwright/test").Page) {
  await page.setViewportSize({ width: 1280, height: 720 });
  await waitForBody(page, "/doc/a/b/c.md");

  const targetScrollY = await page.evaluate(() => {
    window.scrollTo(0, 220);
    return window.scrollY;
  });
  await page.waitForFunction(
    ([key, value]) => window.sessionStorage.getItem(key) === value,
    ["m2h.scroll.a/b/c.md", String(targetScrollY)],
  );

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector('[aria-current="page"]') !== null,
  );
  // The reveal runs before first paint; wait until it has actually scrolled.
  await page.waitForFunction(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    return (
      viewport !== null && viewport !== undefined && viewport.scrollTop > 0
    );
  });
  return targetScrollY;
}

test("centers the capped document inside a wide canvas", async ({ page }) => {
  await page.setViewportSize({ width: 1600, height: 900 });
  await waitForBody(page, "/doc/scroll.md");

  const gaps = await page.evaluate(() => {
    const canvas = document.querySelector(".reader-canvas");
    const article = document.querySelector(".reader-document");
    if (canvas === null || article === null) {
      throw new Error("reader canvas or document was not rendered");
    }
    const canvasRect = canvas.getBoundingClientRect();
    const articleRect = article.getBoundingClientRect();
    return {
      canvasWidth: canvasRect.width,
      left: articleRect.left - canvasRect.left,
      right: canvasRect.right - articleRect.right,
    };
  });

  // 1600px minus the 256px sidebar leaves the canvas far wider than the
  // standard 980px cap, so the cap engages and the margins must match.
  expect(gaps.canvasWidth).toBeGreaterThan(1030);
  expect(Math.abs(gaps.left - gaps.right)).toBeLessThanOrEqual(1);
});

test("offers the outline in a sheet on narrow viewports", async ({ page }) => {
  await page.setViewportSize({ width: 1024, height: 768 });
  await waitForBody(page, "/doc/scroll.md");

  // Below 1200px the rail and its persistent toggle disappear; the toolbar
  // sheet trigger takes over instead of leaving a "pressed but invisible" TOC.
  await expect(page.locator(".reader-toc")).toBeHidden();
  await expect(page.locator(".reader-toc-toggle")).toBeHidden();

  const trigger = page.getByRole("button", { name: "打开文档目录" });
  await expect(trigger).toBeVisible();
  await trigger.click();

  const dialog = page.getByRole("dialog");
  await expect(dialog).toBeVisible();
  const link = dialog.getByRole("link", { name: "目标章节" });
  await expect(link).toBeVisible();
  await link.click();

  // The sheet closes first and hands the body scroll to the next frame; the
  // fragment is then recorded through the shared URL funnel.
  await expect(dialog).toBeHidden();
  await expect
    .poll(() => page.evaluate(() => window.location.hash))
    .toBe(encodeURI("#目标章节"));

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const toolbarBottom = await page.evaluate(() => {
    const toolbar = document.querySelector(".reader-toolbar");
    if (toolbar === null) {
      throw new Error("reader toolbar was not rendered");
    }
    return toolbar.getBoundingClientRect().bottom;
  });
  // The heading lands just below the sticky toolbar, not underneath it.
  await expect
    .poll(() =>
      heading.evaluate((element) => element.getBoundingClientRect().top),
    )
    .toBeGreaterThanOrEqual(toolbarBottom - 1);
  await expect
    .poll(() =>
      heading.evaluate((element) => element.getBoundingClientRect().top),
    )
    .toBeLessThanOrEqual(toolbarBottom + 200);
});

test("reveals the active file in the tree after a reload without moving the reader", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  // The tree fixture holds 24 notes; note-24 sits far below the sidebar's
  // initial fold, so the reload must scroll the tree viewport to reveal it.
  await waitForBody(page, "/doc/tree/note-24.md");

  const targetScrollY = await page.evaluate(() => {
    window.scrollTo(0, 200);
    return window.scrollY;
  });
  await page.waitForFunction(
    ([key, value]) => window.sessionStorage.getItem(key) === value,
    ["m2h.scroll.tree/note-24.md", String(targetScrollY)],
  );

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector('[aria-current="page"]') !== null,
  );

  // The saved reading offset comes back first …
  await expect
    .poll(() => page.evaluate(() => window.scrollY))
    .toBe(targetScrollY);

  // … and the tree reveal brings the active file inside the sidebar viewport.
  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        const active = document.querySelector<HTMLElement>(
          '[aria-current="page"]',
        );
        if (viewport === null || viewport === undefined || active === null) {
          return null;
        }
        const viewportRect = viewport.getBoundingClientRect();
        const activeRect = active.getBoundingClientRect();
        return (
          activeRect.top >= viewportRect.top - 1 &&
          activeRect.bottom <= viewportRect.bottom + 1
        );
      }),
    )
    .toBe(true);

  // The reveal scrolled the tree (not just left it at the top) …
  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        return viewport === null || viewport === undefined
          ? -1
          : viewport.scrollTop;
      }),
    )
    .toBeGreaterThan(0);

  // … and it never touched the reader's restored position.
  expect(await page.evaluate(() => window.scrollY)).toBe(targetScrollY);
});

test("pins ancestor directories above the revealed active file", async ({
  page,
}) => {
  // The fixture nests a/b/ with 24 notes and c.md last, so revealing c.md
  // scrolls the tree well past both ancestor directories.
  const targetScrollY = await openRevealedNestedFile(page);

  const geometry = await page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    const dirA = document.querySelector<HTMLElement>('[data-tree-path="a"]');
    const dirB = document.querySelector<HTMLElement>('[data-tree-path="a/b"]');
    const active = document.querySelector<HTMLElement>('[aria-current="page"]');
    if (
      viewport === undefined ||
      viewport === null ||
      dirA === null ||
      dirB === null ||
      active === null
    ) {
      throw new Error("tree viewport or sticky rows were not rendered");
    }
    const viewportTop = viewport.getBoundingClientRect().top;
    const a = dirA.getBoundingClientRect();
    const b = dirB.getBoundingClientRect();
    const c = active.getBoundingClientRect();
    return {
      aTop: a.top - viewportTop,
      aBottom: a.bottom - viewportTop,
      bTop: b.top - viewportTop,
      bBottom: b.bottom - viewportTop,
      cTop: c.top - viewportTop,
    };
  });

  // Both ancestors stick to the viewport top, stacking exactly one row apart …
  expect(Math.abs(geometry.aTop)).toBeLessThanOrEqual(1);
  expect(Math.abs(geometry.bTop - geometry.aBottom)).toBeLessThanOrEqual(1);
  // … and the revealed file sits clear of the sticky stack, not under it.
  expect(geometry.cTop).toBeGreaterThanOrEqual(geometry.bBottom - 1);

  // The sticky reveal is a tree-viewport-only scroll: the reader keeps the
  // offset restored from sessionStorage.
  expect(await page.evaluate(() => window.scrollY)).toBe(targetScrollY);
});

test("keeps sidebar wheel scrolling isolated from the reader window", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 720 });
  await waitForBody(page, "/doc/tree/note-24.md");

  const readerY = await page.evaluate(() => {
    window.scrollTo(0, 200);
    return window.scrollY;
  });

  // Start from the top of the tree so a downward wheel has room to scroll.
  await setSidebarScrollTop(page, "top");
  const viewport = await readSidebarGeometry(page);
  await page.mouse.move(
    viewport.x + viewport.width / 2,
    viewport.y + viewport.height / 2,
  );

  // A wheel over the tree scrolls the tree viewport …
  await page.mouse.wheel(0, 500);
  await expect
    .poll(async () => (await readSidebarGeometry(page)).scrollTop)
    .toBeGreaterThan(0);
  // … and never the reader window behind it.
  expect(await page.evaluate(() => window.scrollY)).toBe(readerY);

  // Pinned at the bottom edge, further wheels must not scroll-chain into the
  // document — this is what overscroll-behavior-y: contain is for.
  await setSidebarScrollTop(page, "max");
  await page.mouse.wheel(0, 1000);
  await waitForScrollSettle(page);
  expect(await page.evaluate(() => window.scrollY)).toBe(readerY);

  // The same holds at the top edge, wheeling up and out of the tree.
  await setSidebarScrollTop(page, "top");
  await page.mouse.wheel(0, -1000);
  await waitForScrollSettle(page);
  expect(await page.evaluate(() => window.scrollY)).toBe(readerY);
});

test("normalizes the tree viewport after collapsing a sticky ancestor", async ({
  page,
}) => {
  const targetScrollY = await openRevealedNestedFile(page);

  // Collapsing the outermost sticky directory unmounts its whole subtree —
  // including the active file — in a single commit while the viewport is
  // scrolled deep into it.
  await page.locator('[data-tree-path="a"]').click();
  await expect(page.locator('[data-tree-path="a/b"]')).toBeHidden();

  const geometry = await readSidebarGeometry(page);
  // The viewport is re-clamped to the shrunken tree before paint …
  expect(geometry.scrollTop).toBeLessThanOrEqual(
    geometry.scrollHeight - geometry.clientHeight + 1,
  );
  // … keeps no lateral drift and never overflows horizontally at all.
  expect(geometry.scrollLeft).toBe(0);
  expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.clientWidth + 1);

  // The reader keeps the offset restored from sessionStorage.
  expect(await page.evaluate(() => window.scrollY)).toBe(targetScrollY);
});

test("reveals the active file again after re-expanding its collapsed ancestor", async ({
  page,
}) => {
  await openRevealedNestedFile(page);

  await page.locator('[data-tree-path="a"]').click();
  await expect(page.locator('[data-tree-path="a/b"]')).toBeHidden();

  // Re-expanding remounts the subtree with c.md as its last row, far below
  // the fold. The collapse reset the reveal's record, so it must run again
  // and bring the active file back inside the viewport.
  await page.locator('[data-tree-path="a"]').click();
  await expect(page.locator('[aria-current="page"]')).toBeVisible();

  await expect
    .poll(() =>
      page.evaluate(() => {
        const tree = document.querySelector('[aria-label="Markdown 文件树"]');
        const viewport = tree?.closest<HTMLElement>(
          '[data-slot="scroll-area-viewport"]',
        );
        const active = document.querySelector<HTMLElement>(
          '[aria-current="page"]',
        );
        if (viewport === null || viewport === undefined || active === null) {
          return null;
        }
        const viewportRect = viewport.getBoundingClientRect();
        const activeRect = active.getBoundingClientRect();
        return (
          activeRect.top >= viewportRect.top - 1 &&
          activeRect.bottom <= viewportRect.bottom + 1
        );
      }),
    )
    .toBe(true);
});
