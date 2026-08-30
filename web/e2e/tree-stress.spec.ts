import { expect, test } from "@playwright/test";

// Stress regressions for the sidebar-tree collapse bug (stale paint fragments
// along the sidebar's right edge after collapsing a directory). The fixture
// below holds 120 long-named notes across three nesting levels — the sidebar
// then has the same order of magnitude as a real vault, where the bug was
// reproducible only occasionally.
//
// The core scenario is the one the old suite missed: a collapse that changes
// content height while NEITHER a scroll event NOR a viewport resize happens.
// Base UI's Viewport only recomputes its scrollbar/overflow metrics on those
// two triggers; the missing third trigger — content resizing under a static
// viewport — is exactly what ScrollArea.Content's ResizeObserver supplies.

const rootDir = "tree-stress";
const level1 = `${rootDir}/level-1`;
const level2a = `${level1}/level-2-a`;
const level3 = `${level2a}/level-3`;
const deepFile = `${level3}/sticky-compositor-raster-invalidation-note-17.md`;
const rootFile = `${rootDir}/sticky-compositor-raster-invalidation-note-01.md`;

type TreeMetrics = {
  scrollTop: number;
  scrollLeft: number;
  scrollHeight: number;
  clientHeight: number;
  scrollWidth: number;
  clientWidth: number;
  hasOverflowY: boolean;
  overflowYEnd: string;
};

async function readTreeMetrics(
  page: import("@playwright/test").Page,
): Promise<TreeMetrics> {
  return page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    const area = tree?.closest<HTMLElement>('[data-slot="scroll-area"]');
    if (!(viewport instanceof HTMLElement) || !(area instanceof HTMLElement)) {
      throw new Error("tree scroll area was not rendered");
    }
    return {
      scrollTop: viewport.scrollTop,
      scrollLeft: viewport.scrollLeft,
      scrollHeight: viewport.scrollHeight,
      clientHeight: viewport.clientHeight,
      scrollWidth: viewport.scrollWidth,
      clientWidth: viewport.clientWidth,
      hasOverflowY: area.hasAttribute("data-has-overflow-y"),
      overflowYEnd: viewport.style.getPropertyValue(
        "--scroll-area-overflow-y-end",
      ),
    };
  });
}

async function waitForTreePage(
  page: import("@playwright/test").Page,
  path: string,
) {
  await page.goto(`/doc/${path}`);
  await page.waitForFunction(
    () => document.querySelector('[aria-current="page"]') !== null,
  );
}

// Is the active file row back inside the tree viewport (the re-expand reveal)?
async function activeRowInsideViewport(
  page: import("@playwright/test").Page,
): Promise<boolean> {
  return page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    const active = document.querySelector<HTMLElement>('[aria-current="page"]');
    if (viewport === null || viewport === undefined || active === null) {
      return false;
    }
    const viewportRect = viewport.getBoundingClientRect();
    const activeRect = active.getBoundingClientRect();
    return (
      activeRect.top >= viewportRect.top - 1 &&
      activeRect.bottom <= viewportRect.bottom + 1
    );
  });
}

test("re-measures overflow after a collapse that scrolls and resizes nothing", async ({
  page,
}) => {
  // 1440 tall so the fully collapsed tree (the fixture root plus tree-stress's
  // root notes) fits the viewport with margin — the collapse below must end
  // in a no-overflow state for the re-measure assertion to be observable. The
  // height is calibrated against the whole fixture directory: every root-level
  // e2e document adds a sidebar row, and at 1280 the margin was down to a
  // single row; the security fixtures added two more rows since.
  await page.setViewportSize({ width: 1280, height: 1440 });
  await waitForTreePage(page, rootFile);

  // Expand level-1, then level-2-a: 40 more rows render on top of the other
  // fixtures' rows, guaranteed overflow. Nothing scrolls — every expansion
  // happens below the fold at scrollTop 0.
  await page.locator(`[data-tree-path="${level1}"]`).click();
  await page.locator(`[data-tree-path="${level2a}"]`).click();
  await expect(page.locator(`[data-tree-path="${level3}"]`)).toBeVisible();

  // The overflow flag only flips once Content's ResizeObserver has delivered
  // — the same measurement path this test exists to guard, so waiting here
  // both stabilizes the precondition and pre-validates it.
  await expect
    .poll(async () => (await readTreeMetrics(page)).hasOverflowY)
    .toBe(true);

  const before = await readTreeMetrics(page);
  expect(before.scrollTop).toBe(0);
  expect(before.hasOverflowY).toBe(true);
  expect(before.scrollHeight).toBeGreaterThan(before.clientHeight);
  // At scrollTop 0 the whole scroll range is "left to the end".
  expect(before.overflowYEnd).toMatch(/^[1-9][0-9]*\.?[0-9]*px$/);

  // Spy on the viewport's scroll events: the collapse below keeps scrollTop
  // valid (0 <= a much smaller maxScrollTop) and never resizes the viewport
  // box, so neither of Base UI's other two recompute triggers may fire.
  await page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    window.__treeScrollEvents = 0;
    viewport?.addEventListener(
      "scroll",
      () => {
        window.__treeScrollEvents += 1;
      },
      { passive: true },
    );
  });

  // Collapsing level-1 unmounts both subtrees (100 rows) in one commit.
  await page.locator(`[data-tree-path="${level1}"]`).click();
  await expect(page.locator(`[data-tree-path="${level2a}"]`)).toBeHidden();

  // Let any ResizeObserver delivery land before reading the state.
  await expect
    .poll(async () => (await readTreeMetrics(page)).hasOverflowY)
    .toBe(false);

  const after = await readTreeMetrics(page);
  // No scroll happened and the position is untouched …
  const scrollEvents = await page.evaluate(() => window.__treeScrollEvents);
  expect(scrollEvents).toBe(0);
  expect(after.scrollTop).toBe(0);
  // … so only Content's ResizeObserver can have refreshed the metrics: the
  // overflow flag cleared and the edge variable collapsed back to 0px.
  expect(after.overflowYEnd).toBe("0px");
});

test("survives repeated nested collapse and expand cycles", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  // Deep-link a level-3 note: the reveal expands all three ancestors and
  // scrolls the viewport deep under both sticky directory rows.
  await waitForTreePage(page, deepFile);
  await page.waitForFunction(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    return (
      viewport !== null && viewport !== undefined && viewport.scrollTop > 0
    );
  });

  // Alternate between the outer directory (unmounts the whole 100-row
  // subtree) and the nested one (unmounts 72 rows under a sticky ancestor);
  // both collapse paths must normalize before paint, every single round.
  // The collapsed directory's own row stays visible — only its subtree
  // unmounts — so the hidden check targets the level-3 row inside it.
  for (let round = 1; round <= 12; round += 1) {
    const target = round % 2 === 1 ? level1 : level2a;

    await page.locator(`[data-tree-path="${target}"]`).click();
    await expect(page.locator(`[data-tree-path="${level3}"]`)).toBeHidden();

    const collapsed = await readTreeMetrics(page);
    expect(collapsed.scrollTop).toBeGreaterThanOrEqual(0);
    expect(collapsed.scrollTop).toBeLessThanOrEqual(
      collapsed.scrollHeight - collapsed.clientHeight + 1,
    );
    expect(collapsed.scrollLeft).toBe(0);
    expect(collapsed.scrollWidth).toBeLessThanOrEqual(
      collapsed.clientWidth + 1,
    );

    await page.locator(`[data-tree-path="${target}"]`).click();
    await expect(page.locator('[aria-current="page"]')).toBeVisible();
    await expect.poll(() => activeRowInsideViewport(page)).toBe(true);
  }
});

test("leaves no text pixels at the sidebar boundary after a collapse", async ({
  page,
}) => {
  // Same tall viewport as the re-measure test: after the collapse below the
  // whole tree fits, so the strip is scrollbar-free.
  await page.setViewportSize({ width: 1280, height: 1360 });
  await waitForTreePage(page, deepFile);

  // Return to the top, then collapse the whole level-1 subtree in one commit
  // — the operation that used to leave stale fragments along the sidebar's
  // right edge. Any pixel in the strip that is not sidebar background,
  // border or reader background (i.e. ghost text) fails the shot.
  await page.evaluate(() => {
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport instanceof HTMLElement) {
      viewport.scrollTop = 0;
    }
  });
  await page.locator(`[data-tree-path="${level1}"]`).click();
  await expect(page.locator(`[data-tree-path="${level2a}"]`)).toBeHidden();
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );

  // A 24px vertical strip centred on the sidebar's right boundary, spanning
  // the tree viewport's height (below the reader toolbar): deliberately
  // text-free in this state so the baseline stays stable across platforms.
  const clip = await page.evaluate(() => {
    const sidebar = document.querySelector('[data-slot="sidebar-container"]');
    const tree = document.querySelector('[aria-label="Markdown 文件树"]');
    const viewport = tree?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (
      !(sidebar instanceof HTMLElement) ||
      !(viewport instanceof HTMLElement)
    ) {
      throw new Error("sidebar boundary was not rendered");
    }
    const sidebarRect = sidebar.getBoundingClientRect();
    const viewportRect = viewport.getBoundingClientRect();
    return {
      x: sidebarRect.right - 8,
      y: viewportRect.top + 4,
      width: 24,
      height: viewportRect.height - 8,
    };
  });

  await expect(page).toHaveScreenshot("sidebar-boundary-strip.png", {
    clip,
    animations: "disabled",
    caret: "hide",
    maxDiffPixelRatio: 0.02,
  });
});
