import { expect, test } from "@playwright/test";

// Regression guard for the browser's native scroll restoration. m2h no longer
// runs a custom restore loop: the reader scrolls the document viewport (the
// window — the only scroller Chromium keeps retrying while late content
// arrives), history.scrollRestoration stays on "auto", and CSS scroll
// anchoring absorbs late layout changes. jsdom cannot catch any of this — it
// has no layout engine — so this suite locks the contract down in a real
// browser: after a reload the browser itself must bring the reading position
// back to the exact offset it saved.
test("keeps the reading position stable across a reload", async ({ page }) => {
  await page.goto("/doc/scroll.md");

  // Drive the reader to a known depth. If the document were shorter than the
  // target the browser would clamp the value, so the offset to assert against
  // is the clamped one actually reached.
  const targetScrollY = await page.evaluate(() => {
    window.scrollTo(0, 2500);
    return window.scrollY;
  });

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const before = await heading.evaluate(
    (element) => element.getBoundingClientRect().top,
  );

  await page.reload();

  // The browser restores the saved offset — retrying while the async document
  // content arrives; the visible heading must land back at the pre-reload
  // viewport pixel. More than 1px of drift means something interfered with the
  // native restoration.
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
