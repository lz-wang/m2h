import { expect, test } from "@playwright/test";

// Real-browser regressions for collapsed overlong code blocks. The fold is
// pure CSS layout (max-height in lh units + overflow clipping) and the toggle
// flips data attributes the stylesheet keys on — jsdom computes no geometry,
// so the collapsed/expanded heights and the surviving horizontal scroll can
// only be locked down with a genuine layout engine.

// Wait until the document body has painted, so geometry assertions never race
// the client-rendered article (same helper contract as layout.spec.ts). The
// fixture holds a Mermaid diagram, so also wait for its source block to be
// replaced: pre-counting assertions must not observe the transient <pre>.
async function waitForBody(
  page: import("@playwright/test").Page,
  path: string,
) {
  await page.goto(path);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body p, .markdown-body h2") !== null &&
      document.querySelector("pre > code.language-mermaid") === null,
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
  await expect(toggle).toHaveText("展开代码（共120行）");
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
  await expect(toggle).toHaveText("展开代码（共120行）");
  const refolded = await measureBlock(page, 1);
  expect(refolded.datasetCollapsed).toBe("true");
  expect(refolded.clientHeight).toBe(collapsed.clientHeight);
});

test("numbers code lines in a gutter that stays pinned while scrolling", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await waitForBody(page, "/doc/long-code.md");

  // Every fenced source block carries a gutter with exactly one number per
  // source line; the trailing newline never materializes as an extra number.
  const blocks = await page.evaluate(() =>
    Array.from(document.querySelectorAll<HTMLElement>("pre")).map((pre) => ({
      numbers: pre.querySelectorAll(":scope > .m2h-code-line-numbers > span")
        .length,
      withLines: pre.classList.contains("m2h-code-with-lines"),
    })),
  );
  expect(blocks).toEqual([
    { numbers: 3, withLines: true },
    { numbers: 25, withLines: true },
    { numbers: 26, withLines: true },
    { numbers: 120, withLines: true },
  ]);

  // The rendered diagram is never numbered.
  expect(
    await page.evaluate(
      () => document.querySelectorAll(".mermaid .m2h-code-line-numbers").length,
    ),
  ).toBe(0);

  // Pixel lock-step: the gutter inherits the pre's font metrics, so across
  // 120 lines the number column and the code column end the same height and
  // the first number is level with the first source line. A divergent
  // font-size or line-height accumulates tens of pixels of drift instead.
  const alignment = await page.evaluate(() => {
    const pre = document.querySelectorAll<HTMLElement>("pre")[3];
    if (pre === undefined) {
      throw new Error("120-line block was not rendered");
    }
    const gutter = pre.querySelector<HTMLElement>(
      ":scope > .m2h-code-line-numbers",
    );
    const code = pre.querySelector<HTMLElement>(":scope > code");
    if (gutter === null || code === null) {
      throw new Error("gutter or code element was not rendered");
    }
    return {
      gutterHeight: gutter.getBoundingClientRect().height,
      codeHeight: code.getBoundingClientRect().height,
      firstNumberTop:
        gutter.firstElementChild?.getBoundingClientRect().top ?? -1,
      codeTop: code.getBoundingClientRect().top,
    };
  });
  expect(Math.abs(alignment.gutterHeight - alignment.codeHeight)).toBeLessThan(
    2,
  );
  expect(Math.abs(alignment.firstNumberTop - alignment.codeTop)).toBeLessThan(
    2,
  );

  // Scrolling the long line sideways keeps the numbers pinned at the block's
  // left edge instead of scrolling away with the content.
  const sticky = await page.evaluate(() => {
    const pre = document.querySelectorAll<HTMLElement>("pre")[3];
    if (pre === undefined) {
      throw new Error("120-line block was not rendered");
    }
    pre.scrollLeft = 300;
    const gutter = pre.querySelector<HTMLElement>(
      ":scope > .m2h-code-line-numbers",
    );
    if (gutter === null) {
      throw new Error("gutter was not rendered");
    }
    const preRect = pre.getBoundingClientRect();
    const gutterRect = gutter.getBoundingClientRect();
    return {
      scrollLeft: pre.scrollLeft,
      leftGap: gutterRect.left - preRect.left,
      visible: gutterRect.right > preRect.left,
    };
  });
  expect(sticky.scrollLeft).toBeGreaterThan(0);

  /*
   * The sticky gutter must cover the scrollport's actual left edge. A positive
   * ~16px gap means source text can leak through the pre's former left padding.
   */
  expect(Math.abs(sticky.leftGap)).toBeLessThanOrEqual(1);

  expect(sticky.visible).toBe(true);

  // Layout responsibility invariant: the numbered pre owns no left inset; the
  // gutter's own padding provides it. Locking both computed values keeps a
  // future github-markdown-css upgrade (or stylesheet edit) from silently
  // reintroducing the uncovered strip beside the gutter.
  const paddingGeometry = await page.evaluate(() => {
    const pre = document.querySelector<HTMLElement>("pre.m2h-code-with-lines");
    const gutter = pre?.querySelector<HTMLElement>(
      ":scope > .m2h-code-line-numbers",
    );
    if (pre === null || gutter === null) {
      throw new Error("numbered code block was not rendered");
    }
    return {
      prePaddingLeft: getComputedStyle(pre).paddingLeft,
      gutterPaddingLeft: getComputedStyle(gutter).paddingLeft,
    };
  });
  expect(paddingGeometry.prePaddingLeft).toBe("0px");
  expect(paddingGeometry.gutterPaddingLeft).toBe("16px");

  // The gutter keeps its opaque plate and padding but no divider line.
  expect(
    await page.evaluate(() => {
      const gutter = document.querySelector<HTMLElement>(
        ".m2h-code-line-numbers",
      );
      if (gutter === null) {
        throw new Error("gutter was not rendered");
      }
      return getComputedStyle(gutter).borderRightWidth;
    }),
  ).toBe("0px");
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
