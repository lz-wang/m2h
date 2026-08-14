import { expect, test } from "@playwright/test";

// Regression guard for the reload scroll restore. The bug class has now
// appeared twice: an async rich-content enhancement (Mermaid, KaTeX, and most
// recently Tablesort) changes the body geometry after the restore already
// landed, so the visible content drifts even though the saved offset was
// written correctly. jsdom cannot catch this — it has no layout engine — so
// this suite locks the invariant down in a real browser: after a reload, the
// same heading must sit at the same viewport pixel, with the sortable-table
// runtime deliberately delayed past first paint.
const scrollStorageKey = "m2h.scroll.scroll.md";

test("keeps the reading position stable across a reload with async sortable tables", async ({
  page,
}) => {
  // Delay every Tablesort script so the enhancement always lands well after
  // first paint, maximizing the window in which a timing-based restore could
  // be overtaken by the table reflow.
  await page.route("**/runtime/tablesort*.js", async (route) => {
    await new Promise((resolve) => setTimeout(resolve, 100));
    await route.continue();
  });

  await page.goto("/doc/scroll.md");
  const viewport = page.locator(
    '.reader-main [data-slot="scroll-area-viewport"]',
  );
  await expect(viewport).toBeAttached();

  // First load: wait for the delayed enhancement to finish before measuring,
  // so the pre-reload position reflects the fully enhanced page.
  await page.waitForFunction(
    () => document.querySelector('thead th[role="columnheader"]') !== null,
  );
  // Then wait for the initial restore to release its guard: the release
  // dispatches a scroll event that makes the saver write the current (top)
  // offset. Scrolling earlier would be ignored by the still-held guard.
  await page.waitForFunction(
    (key) => window.sessionStorage.getItem(key) !== null,
    scrollStorageKey,
  );

  // Read the offset back: if the document were shorter than the target, the
  // browser would clamp the value, and the stored/restored offset to assert
  // against is the clamped one.
  const targetScrollTop = await viewport.evaluate((element) => {
    element.scrollTop = 2500;
    return element.scrollTop;
  });
  // The rAF-throttled saver persists the new offset within a frame; waiting
  // for the stored value avoids racing the reload against the save.
  await page.waitForFunction(
    ([key, value]) => window.sessionStorage.getItem(key) === value,
    [scrollStorageKey, String(targetScrollTop)],
  );

  const heading = page.locator(".markdown-body h2", { hasText: "目标章节" });
  const before = await heading.evaluate(
    (element) => element.getBoundingClientRect().top,
  );

  await page.reload();

  // The restore runs once the (still delayed) table enhancement has settled
  // and the reader height has been stable for several frames. Poll until the
  // heading is back at the pre-reload pixel; layout drift of more than 1px
  // means the enhancement changed geometry after the restore landed.
  await expect
    .poll(async () => {
      const after = await heading.evaluate(
        (element) => element.getBoundingClientRect().top,
      );
      return Math.abs(after - before);
    })
    .toBeLessThanOrEqual(1);
  await expect
    .poll(async () => viewport.evaluate((element) => element.scrollTop))
    .toBe(targetScrollTop);
});
