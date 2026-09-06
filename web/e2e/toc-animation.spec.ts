import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for the desktop TOC's expand/collapse transition.
// The rail is a persistent DOM node whose slot animates its layout width —
// the same contract as the sidebar's gap + container pair — so these tests
// pin the lifecycle (no unmount across toggles), the geometry (0 ↔ 15rem),
// keyboard retirement while collapsed (inert), the animation parameters
// shared with the sidebar, and the absence of a page-level horizontal
// scrollbar in every state.

const RAIL_WIDTH = 240; // 15rem

async function openDocument(page: Page) {
  await page.setViewportSize({ width: 1440, height: 900 });
  // scroll.md holds one H2, so the desktop rail renders expanded by default.
  await page.goto("/doc/scroll.md");
  await page.waitForFunction(
    () =>
      document.querySelector(".reader-toc-slot")?.getAttribute("data-state") ===
      "expanded",
  );
}

function expectNoHorizontalOverflow(page: Page) {
  return expect
    .poll(() =>
      page.evaluate(
        () =>
          document.documentElement.scrollWidth -
          document.documentElement.clientWidth,
      ),
    )
    .toBeLessThanOrEqual(0);
}

test("keeps the rail mounted while collapsing and expanding the slot", async ({
  page,
}) => {
  await openDocument(page);

  // The exact DOM nodes this test follows across the whole round trip.
  const slot = await page.evaluateHandle(() =>
    document.querySelector(".reader-toc-slot"),
  );
  const rail = await page.evaluateHandle(() =>
    document.querySelector(".reader-toc"),
  );
  const slotWidth = () =>
    slot.evaluate((node) =>
      node instanceof HTMLElement ? node.getBoundingClientRect().width : -1,
    );

  await expect.poll(slotWidth).toBe(RAIL_WIDTH);
  await expectNoHorizontalOverflow(page);

  await page.getByRole("button", { name: "隐藏文档目录" }).click();

  // The slot animates to zero but the DOM never unmounts, and the collapsed
  // slot retires itself from the accessibility tree.
  await expect.poll(slotWidth).toBe(0);
  expect(await slot.evaluate((node) => node.isConnected)).toBe(true);
  expect(await rail.evaluate((node) => node.isConnected)).toBe(true);
  await expect(page.locator(".reader-toc-slot")).toHaveAttribute(
    "data-state",
    "collapsed",
  );
  await expect(page.locator(".reader-toc-slot")).toHaveAttribute(
    "aria-hidden",
    "true",
  );
  await expectNoHorizontalOverflow(page);

  // Collapsed: inert keeps the rail's links out of the tab order — focus()
  // is a no-op on an inert element.
  expect(
    await page.evaluate(() => {
      const link =
        document.querySelector<HTMLAnchorElement>(".reader-toc-link");
      if (link === null) {
        throw new Error("TOC link was not rendered");
      }
      link.focus();
      return document.activeElement === link;
    }),
  ).toBe(false);

  await page.getByRole("button", { name: "显示文档目录" }).click();

  // Re-expanding restores the width on the very same nodes.
  await expect.poll(slotWidth).toBe(RAIL_WIDTH);
  expect(await slot.evaluate((node) => node.isConnected)).toBe(true);
  expect(await rail.evaluate((node) => node.isConnected)).toBe(true);
  await expect(page.locator(".reader-toc-slot")).toHaveAttribute(
    "aria-hidden",
    "false",
  );
  expect(
    await page.evaluate(() => {
      const link =
        document.querySelector<HTMLAnchorElement>(".reader-toc-link");
      if (link === null) {
        throw new Error("TOC link was not rendered");
      }
      link.focus();
      return document.activeElement === link;
    }),
  ).toBe(true);
  await expectNoHorizontalOverflow(page);
});

test("moves the floating navigation with the rail, not ahead of it", async ({
  page,
}) => {
  await openDocument(page);

  // Expanded: the pair sits left of the rail with the standard 1.5rem gap.
  await expect
    .poll(() =>
      page.evaluate(() => {
        const nav = document.querySelector(".reader-navigation");
        const slot = document.querySelector(".reader-toc-slot");
        if (nav === null || slot === null) {
          throw new Error("navigation or TOC slot was not rendered");
        }
        return Math.round(
          slot.getBoundingClientRect().left - nav.getBoundingClientRect().right,
        );
      }),
    )
    .toBe(24);

  await page.getByRole("button", { name: "隐藏文档目录" }).click();

  // Collapsed: the pair returns to the viewport's right edge (1.5rem inset)
  // once both the slot width and the navigation offset settle.
  await expect
    .poll(() =>
      page.evaluate(() => {
        const nav = document.querySelector(".reader-navigation");
        if (nav === null) {
          throw new Error("reader navigation was not rendered");
        }
        return Math.round(
          window.innerWidth - nav.getBoundingClientRect().right,
        );
      }),
    )
    .toBe(24);
  await expectNoHorizontalOverflow(page);
});

test("shares the sidebar's 200ms linear transition contract", async ({
  page,
}) => {
  await openDocument(page);

  const params = await page.evaluate(() => {
    const read = (selector: string) => {
      const element = document.querySelector(selector);
      if (element === null) {
        throw new Error(`${selector} was not rendered`);
      }
      const style = getComputedStyle(element);
      return {
        durations: style.transitionDuration.split(",").map((v) => v.trim()),
        timings: style.transitionTimingFunction.split(",").map((v) => v.trim()),
      };
    };
    return {
      "sidebar gap": read('[data-slot="sidebar-gap"]'),
      "sidebar container": read('[data-slot="sidebar-container"]'),
      "toc slot": read(".reader-toc-slot"),
      "toc rail": read(".reader-toc"),
      "reader navigation": read(".reader-navigation"),
    };
  });

  for (const [name, { durations, timings }] of Object.entries(params)) {
    // Every animated property of every layer uses the same 200ms linear, so
    // the sidebar and the TOC can never move at different speeds.
    expect(
      durations.every((duration) => duration === "0.2s"),
      `${name}: ${durations.join(", ")}`,
    ).toBe(true);
    expect(
      timings.every((timing) => timing === "linear"),
      `${name}: ${timings.join(", ")}`,
    ).toBe(true);
  }
});

test("snaps between states under prefers-reduced-motion", async ({ page }) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await openDocument(page);

  const slotWidth = () =>
    page.evaluate(() => {
      const slot = document.querySelector(".reader-toc-slot");
      return slot === null ? -1 : slot.getBoundingClientRect().width;
    });

  await expect.poll(slotWidth).toBe(RAIL_WIDTH);
  await page.getByRole("button", { name: "隐藏文档目录" }).click();
  // The transition is disabled entirely, so the rail lands at zero width
  // without a 200ms flight.
  await expect.poll(slotWidth).toBe(0);
  await page.getByRole("button", { name: "显示文档目录" }).click();
  await expect.poll(slotWidth).toBe(RAIL_WIDTH);
  await expectNoHorizontalOverflow(page);
});

test("keeps the desktop TOC pinned when the document reaches its bottom", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  // toc-scroll.md scrolls well past the fold, so the rail's viewport pinning
  // is exercised across the full scroll range instead of only near the top.
  await page.goto("/doc/toc-scroll.md");
  await page.waitForFunction(
    () =>
      document.querySelector(".reader-toc-slot")?.getAttribute("data-state") ===
      "expanded",
  );

  // The invariant under test: the rail hangs off the toolbar's bottom edge
  // and reaches down to the viewport's bottom edge at every scroll offset.
  // The larger of the two gaps must stay within a pixel — a sticky layout
  // box violates this near the document's end, where its bottom constraint
  // drags the rail upward.
  const worstGap = () =>
    page.evaluate(() => {
      const toolbar = document.querySelector(".reader-toolbar");
      const rail = document.querySelector(".reader-toc");
      if (toolbar === null || rail === null) {
        throw new Error("toolbar or TOC rail was not rendered");
      }
      const railRect = rail.getBoundingClientRect();
      return Math.max(
        Math.abs(railRect.top - toolbar.getBoundingClientRect().bottom),
        Math.abs(document.documentElement.clientHeight - railRect.bottom),
      );
    });

  await expect.poll(worstGap).toBeLessThanOrEqual(1);

  // Mid-document: any scroll offset between the top and the end.
  await page.evaluate(() => {
    const max = document.documentElement.scrollHeight - window.innerHeight;
    window.scrollTo(0, max / 2);
  });
  await expect.poll(worstGap).toBeLessThanOrEqual(1);

  // The document's very end is where any slot-bound rail feels the pull, so
  // scroll there and let two animation frames flush layout before checking.
  await page.evaluate(() => {
    window.scrollTo(0, Number.MAX_SAFE_INTEGER);
  });
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
  await expect.poll(worstGap).toBeLessThanOrEqual(1);
  await expectNoHorizontalOverflow(page);

  // In-flow tail content after .reader-main (the copy status line renders
  // exactly there) is the case a sticky slot cannot survive: its bottom
  // constraint drags the rail up by the tail's full height, while a
  // viewport-fixed rail must not move at all.
  await page.evaluate(() => {
    const spacer = document.createElement("div");
    spacer.style.height = "200px";
    document.querySelector(".reader-inset")?.append(spacer);
  });
  await page.evaluate(() => {
    window.scrollTo(0, Number.MAX_SAFE_INTEGER);
  });
  await page.evaluate(
    () =>
      new Promise<void>((resolve) => {
        requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
      }),
  );
  await expect.poll(worstGap).toBeLessThanOrEqual(1);
});
