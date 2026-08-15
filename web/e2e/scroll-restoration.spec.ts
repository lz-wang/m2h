import { expect, test } from "@playwright/test";

// Regression guards for the reading-position restore. The reader scrolls the
// document viewport (the window) and the tab remembers the exact offset in
// sessionStorage (the browser's own restoration was measured not to fire for
// this client-rendered shape); late reflows are left to CSS scroll anchoring.
// jsdom cannot catch any of this — it has no layout engine — so this suite
// locks the contract down in a real browser.

const storageKey = "m2h.scroll.scroll.md";
const imagesStorageKey = "m2h.scroll.images.md";

// Wait until the saver has persisted the given offset, so a reload can never
// race the rAF-throttled write.
async function waitForSavedOffset(
  page: import("@playwright/test").Page,
  key: string,
  offset: number,
) {
  await page.waitForFunction(
    ([k, v]) => window.sessionStorage.getItem(k) === v,
    [key, String(offset)],
  );
}

test("keeps the reading position stable across a reload", async ({ page }) => {
  await page.goto("/doc/scroll.md");
  await page.waitForFunction(
    () => document.querySelector(".markdown-body h2") !== null,
  );

  // Drive the reader to a known depth once the body exists. If the document
  // were shorter than the target the browser would clamp the value, so the
  // offset to assert against is the clamped one actually reached.
  const targetScrollY = await page.evaluate(() => {
    window.scrollTo(0, 2500);
    return window.scrollY;
  });
  await waitForSavedOffset(page, storageKey, targetScrollY);

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const before = await heading.evaluate(
    (element) => element.getBoundingClientRect().top,
  );

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector(".markdown-body h2") !== null,
  );

  // The saved offset is restored once the document commits, and the visible
  // heading must land back at the pre-reload viewport pixel. More than 1px of
  // drift means the restore raced the async content.
  await expect
    .poll(() => page.evaluate(() => window.scrollY))
    .toBe(targetScrollY);
  await expect
    .poll(async () => {
      const after = await heading.evaluate(
        (element) => element.getBoundingClientRect().top,
      );
      return Math.abs(after - before);
    })
    .toBeLessThanOrEqual(1);
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
  // pre-reload position reflects the fully laid-out page.
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body img")?.complete === true &&
      (document.querySelector(".markdown-body img")?.naturalWidth ?? 0) > 0,
  );

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const targetScrollY = await page.evaluate(() => {
    const heading = document.querySelector(".markdown-body h2");
    const headingTop =
      heading === null
        ? 0
        : heading.getBoundingClientRect().top + window.scrollY;
    window.scrollTo(0, Math.max(0, headingTop - 300));
    return window.scrollY;
  });
  await waitForSavedOffset(page, imagesStorageKey, targetScrollY);
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
    .toBeLessThanOrEqual(1);
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
