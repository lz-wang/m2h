import { expect, test } from "@playwright/test";

// Real-browser regressions for collapsed overlong code blocks. The fold is
// pure CSS layout (max-height in lh units + overflow clipping) and the toggle
// flips data attributes the stylesheet keys on — jsdom computes no geometry,
// so the collapsed/expanded heights and the surviving horizontal scroll can
// only be locked down with a genuine layout engine.

// Wait until the document body has painted, so geometry assertions never race
// the client-rendered article (same helper contract as layout.spec.ts).
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

test("collapses overlong code blocks behind an expand toggle", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  // The fixture holds four blocks: 3 lines, exactly 25, exactly 26, and 120
  // lines (with one very long line for the horizontal-scroll assertion).
  await waitForBody(page, "/doc/long-code.md");

  // Only the >25-line blocks get a wrapper, both collapsed by default; the
  // short and exactly-25-line blocks stay bare <pre>.
  const blocks = await page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLElement>(".m2h-code-block")).map(
      (wrapper) => ({
        lineCount: wrapper.dataset.lineCount,
        collapsed: wrapper.dataset.collapsed,
      }),
    ),
  );
  expect(blocks).toEqual([
    { lineCount: "26", collapsed: "true" },
    { lineCount: "120", collapsed: "true" },
  ]);
  const barePres = await page.evaluate(() => {
    const pres = document.querySelectorAll("pre");
    return (
      pres.length - document.querySelectorAll(".m2h-code-block pre").length
    );
  });
  expect(barePres).toBe(2);

  // Geometry of the 120-line block (index 1): the fold clips vertically …
  const collapsed = await measureBlock(page, 1);
  expect(collapsed.clientHeight).toBeLessThan(collapsed.scrollHeight);
  // … but the long line still overflows horizontally and stays scrollable —
  // the collapse must never trade away horizontal scrolling.
  expect(collapsed.scrollWidth).toBeGreaterThan(collapsed.clientWidth);
  const scrolledLeft = await page.evaluate(() => {
    const pre = document.querySelectorAll<HTMLElement>(
      ".m2h-code-block pre",
    )[1];
    if (pre === undefined) {
      throw new Error("collapsed code block was not rendered");
    }
    pre.scrollLeft = 200;
    return pre.scrollLeft;
  });
  expect(scrolledLeft).toBeGreaterThan(0);

  // The toggle announces the full line count, the copy control stays put …
  const block = page.locator(".m2h-code-block").nth(1);
  const toggle = block.locator(".m2h-code-toggle");
  await expect(toggle).toBeVisible();
  await expect(toggle).toHaveText("展开代码 · 120 行");
  await expect(toggle).toHaveAttribute("aria-expanded", "false");
  await expect(block.locator(".m2h-code-copy")).toBeVisible();

  // … expanding reveals the whole block …
  await toggle.click();
  await expect(toggle).toHaveText("折叠代码");
  await expect(toggle).toHaveAttribute("aria-expanded", "true");
  const expanded = await measureBlock(page, 1);
  expect(expanded.datasetCollapsed).toBe("false");
  expect(Math.abs(expanded.clientHeight - expanded.scrollHeight)).toBeLessThan(
    2,
  );

  // … and collapsing again restores the folded height.
  await toggle.click();
  await expect(toggle).toHaveText("展开代码 · 120 行");
  const refolded = await measureBlock(page, 1);
  expect(refolded.datasetCollapsed).toBe("true");
  expect(refolded.clientHeight).toBe(collapsed.clientHeight);
});

interface BlockGeometry {
  clientHeight: number;
  scrollHeight: number;
  clientWidth: number;
  scrollWidth: number;
  datasetCollapsed: string | undefined;
}

async function measureBlock(
  page: import("@playwright/test").Page,
  index: number,
): Promise<BlockGeometry> {
  return page.evaluate((position) => {
    const pre = document.querySelectorAll<HTMLElement>(".m2h-code-block pre")[
      position
    ];
    if (pre === undefined) {
      throw new Error(`code block at index ${position} was not rendered`);
    }
    return {
      clientHeight: pre.clientHeight,
      scrollHeight: pre.scrollHeight,
      clientWidth: pre.clientWidth,
      scrollWidth: pre.scrollWidth,
      datasetCollapsed: pre.closest(".m2h-code-block")?.dataset.collapsed,
    };
  }, index);
}
