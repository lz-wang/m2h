import { expect, test } from "@playwright/test";

// Regression guards for the reading-position restore. The reader scrolls the
// document viewport (the window) and the tab remembers the exact offset in
// sessionStorage (the browser's own restoration was measured not to fire for
// this client-rendered shape); late reflows are left to CSS scroll anchoring.
// jsdom cannot catch any of this — it has no layout engine — so this suite
// locks the contract down in a real browser.

const storageKey = "m2h.scroll.scroll.md";
const imagesStorageKey = "m2h.scroll.images.md";

// The reader page keeps settling long after its content arrives: the sidebar
// loads concurrently with the body, the real font swaps in over the fallback,
// and a lazy image's frame reflows once more as its reveal completes — each
// layer trails the load event by up to seconds. A baseline measured mid-
// settle describes a different layout than the reloaded page (where the same
// layers settle in a different order), so both tests wait out every layer
// before locking one in.

// Wait until the body's geometry stops changing: scroll offset, image height
// and total body height sampled together, identical across four consecutive
// rounds (~1.2s). The image reveal alone can reflow the frame a couple of
// seconds after the load event, so a short quiet window would just race it.
async function waitForStableLayout(page: import("@playwright/test").Page) {
  let previous: string | null = null;
  let matches = 0;
  for (;;) {
    const signature = await page.evaluate(() => {
      const image = document.querySelector(".markdown-body img");
      return [
        window.scrollY,
        image?.getBoundingClientRect().height ?? null,
        document.querySelector(".markdown-body")?.getBoundingClientRect()
          .height ?? null,
      ].join("|");
    });
    matches = signature === previous ? matches + 1 : 0;
    if (matches === 3) {
      return;
    }
    previous = signature;
    await page.waitForTimeout(400);
  }
}

// Wait until the saver has persisted the live scroll position and stopped
// changing, and return that offset. The rAF-throttled saver trails behind a
// scroll-anchoring nudge, so the value it last wrote can lag the viewport;
// asserting against the requested offset instead would fail on the browser's
// own compensation.
async function waitForSettledSave(
  page: import("@playwright/test").Page,
  key: string,
): Promise<number> {
  let previous: number | null = null;
  for (;;) {
    const saved = await page.evaluate((k) => {
      const raw = window.sessionStorage.getItem(k);
      return raw === null
        ? null
        : { value: Number(raw), scrollY: window.scrollY };
    }, key);
    if (
      saved !== null &&
      saved.value === saved.scrollY &&
      saved.value === previous
    ) {
      return saved.value;
    }
    previous = saved?.value ?? null;
    await page.waitForTimeout(200);
  }
}

test("keeps the reading position stable across a reload", async ({ page }) => {
  await page.goto("/doc/scroll.md");
  await page.waitForFunction(
    () => document.querySelector(".markdown-body h2") !== null,
  );
  // The body renders with a fallback font and reflows a couple of pixels when
  // the real one lands; a baseline measured before the swap describes a
  // different layout than the reloaded page, so settle the font first.
  await page.evaluate(() => document.fonts.ready);
  await waitForStableLayout(page);

  // Drive the reader to a known depth once the body exists. If the document
  // were shorter than the target the browser would clamp the value; either
  // way the restore is asserted against the offset the saver actually kept.
  await page.evaluate(() => {
    window.scrollTo(0, 2500);
  });
  const savedOffset = await waitForSettledSave(page, storageKey);

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const before = await heading.evaluate(
    (element) => element.getBoundingClientRect().top,
  );

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector(".markdown-body h2") !== null,
  );

  // The saved offset is restored once the document commits, and the visible
  // heading must land back at the pre-reload viewport pixel. Only a restore
  // that raced the async content drifts far enough to trip the budget below.
  await expect
    .poll(() => page.evaluate(() => window.scrollY))
    .toBe(Number(savedOffset));
  await expect
    .poll(async () => {
      const after = await heading.evaluate(
        (element) => element.getBoundingClientRect().top,
      );
      return Math.abs(after - before);
    })
    // Two pixels, not one: with the body and the sidebar loading concurrently
    // the two loads settle through different layer orders, and the final
    // layout itself can differ by a couple of pixels between them. A restore
    // that raced the async content misses by tens of pixels, far past this.
    .toBeLessThanOrEqual(2);
});

test("keeps the reading position once a late image reflows the body", async ({
  page,
}) => {
  // Delay the image past the reload, so the restore always lands while the
  // body is still 160px short of its final height. With no custom stabilize
  // loop, this is exactly where CSS scroll anchoring must hold the position:
  // the image sits above the viewport and its late arrival must not push the
  // visible heading down.
  await page.route("**/assets/images/tall-banner.png", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 1200));
    await route.continue();
  });

  await page.goto("/doc/images.md");
  // First load: wait for the (still delayed) image before measuring, so the
  // pre-reload position reflects the fully laid-out page — fonts included,
  // whose late swap would otherwise shift the heading between the two runs.
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body img")?.complete === true &&
      (document.querySelector(".markdown-body img")?.naturalWidth ?? 0) > 0,
  );
  await page.evaluate(() => document.fonts.ready);
  await waitForStableLayout(page);

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  await page.evaluate(() => {
    const heading = document.querySelector(".markdown-body h2");
    const headingTop =
      heading === null
        ? 0
        : heading.getBoundingClientRect().top + window.scrollY;
    window.scrollTo(0, Math.max(0, headingTop - 300));
  });
  await waitForSettledSave(page, imagesStorageKey);
  const before = await heading.evaluate(
    (element) => element.getBoundingClientRect().top,
  );

  await page.reload();

  // The image must eventually load again, and once the late reflow settles the
  // visible heading must be back at the pre-reload viewport pixel.
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body img")?.complete === true &&
      (document.querySelector(".markdown-body img")?.naturalWidth ?? 0) > 0,
  );
  await expect
    .poll(async () => {
      const after = await heading.evaluate(
        (element) => element.getBoundingClientRect().top,
      );
      return Math.abs(after - before);
    })
    // Same two-pixel budget as above: the reload settles its layers in a
    // different order and lands on a layout a couple of pixels away from the
    // pre-reload one. Only a failed restore or failed scroll anchoring —
    // double-digit drift — may turn this red.
    .toBeLessThanOrEqual(2);
});

test("lands a fresh #hash navigation on the heading, not on a scroll offset", async ({
  page,
}) => {
  // A tab with its own scroll state: the reader sits partway down.
  await page.goto("/doc/scroll.md");
  await page.waitForFunction(
    () => document.querySelector(".markdown-body h2") !== null,
  );
  await page.evaluate(() => {
    window.scrollTo(0, 300);
    return window.scrollY;
  });

  // A new window on the same document with a fragment shares no saved offset,
  // so it must position from the URL: the heading lands just below the sticky
  // toolbar, not wherever the first tab was.
  const fresh = await page.context().newPage();
  await fresh.goto("/doc/scroll.md#目标章节");
  const heading = fresh.locator(".markdown-body h2", { hasText: "目标章节" });
  await expect(heading).toBeVisible();

  const bar = await fresh.evaluate(() => {
    const toolbar = document.querySelector(".reader-toolbar");
    return toolbar === null ? 0 : toolbar.getBoundingClientRect().bottom;
  });
  await expect
    .poll(() =>
      heading.evaluate((element) => element.getBoundingClientRect().top),
    )
    .toBeGreaterThanOrEqual(bar - 1);
  await expect
    .poll(() =>
      heading.evaluate((element) => element.getBoundingClientRect().top),
    )
    .toBeLessThanOrEqual(bar + 96);
  // The landing scrolled the window to the section, not to the first tab's
  // offset of 300 — and the fragment survives in the URL.
  await expect
    .poll(() => fresh.evaluate(() => window.scrollY))
    .toBeGreaterThan(2000);
  expect(fresh.url().split("#")[1] ?? "").toBe(encodeURI("目标章节"));
});
