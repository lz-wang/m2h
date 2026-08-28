import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for the small-image name tooltip. Everything here
// needs a genuine layout engine: the tooltip is an absolutely-positioned label
// inside a 16px frame, and the two failure modes under test — the containing
// block squeezing the label's shrink-to-fit width, and an edge-adjacent label
// pushing a page-level horizontal scrollbar — are invisible to jsdom, which
// computes no geometry at all.

const documentPath = "/doc/image-tooltip.md";

// The fixture places the same 16×16 image three times: document left (short
// name), body middle (long name), table right near the body's right edge.
const frameCount = 3;

async function openDocument(page: Page, query = "") {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(`${documentPath}${query}`);
  await page.waitForFunction(
    (count) =>
      document.querySelectorAll(".m2h-image-frame").length === count &&
      document.querySelectorAll(".m2h-image-name-tooltip").length === count,
    frameCount,
  );
}

interface TooltipGeometry {
  imageWidth: number;
  tooltipWidth: number;
}

async function measureTooltip(
  page: Page,
  frameIndex: number,
): Promise<TooltipGeometry> {
  return page.evaluate((index) => {
    const frame =
      document.querySelectorAll<HTMLElement>(".m2h-image-frame")[index];
    const image = frame?.querySelector("img");
    const tooltip = frame?.querySelector<HTMLElement>(
      ".m2h-image-name-tooltip",
    );
    if (frame === undefined || image === null || tooltip === null) {
      throw new Error(`tooltip geometry unavailable for frame ${index}`);
    }
    return {
      imageWidth: image.getBoundingClientRect().width,
      tooltipWidth: tooltip.getBoundingClientRect().width,
    };
  }, frameIndex);
}

// The page must never grow a horizontal scrollbar because of the tooltips:
// they are always in layout (opacity only), so an edge-adjacent label that
// escapes the viewport shows up in scrollWidth permanently, not just on hover.
async function expectNoHorizontalOverflow(page: Page) {
  const overflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth -
      document.documentElement.clientWidth,
  );
  expect(overflow).toBeLessThanOrEqual(0);
}

for (const [label, query] of [
  ["standard width", ""],
  ["full width", "?width=full"],
] as const) {
  test(`sizes every tooltip to its label instead of the 16px image (${label})`, async ({
    page,
  }) => {
    await openDocument(page, query);

    for (let index = 0; index < frameCount; index += 1) {
      const geometry = await measureTooltip(page, index);
      // The label's width is decided by its text (up to the cap), never
      // squeezed to the icon-sized image it floats over.
      expect(geometry.tooltipWidth).toBeGreaterThan(geometry.imageWidth);
      // 20rem cap; the long middle name reaches it, the short names stay
      // below, so this also pins the ellipsis ceiling.
      expect(geometry.tooltipWidth).toBeLessThanOrEqual(320);
    }

    await expectNoHorizontalOverflow(page);
  });
}

test("caps the long name at 20rem while the short name stays under it", async ({
  page,
}) => {
  await openDocument(page);

  const shortName = await measureTooltip(page, 0);
  const longName = await measureTooltip(page, 1);

  expect(shortName.tooltipWidth).toBeGreaterThan(16);
  expect(shortName.tooltipWidth).toBeLessThan(320);
  // The long fixture name far exceeds 20rem of text, so the cap — not the
  // text — decides its width.
  expect(longName.tooltipWidth).toBe(320);
});

test("reveals the tooltip on frame hover", async ({ page }) => {
  await openDocument(page);

  const frame = page.locator(".m2h-image-frame").first();
  await frame.hover();

  const tooltip = page.locator(".m2h-image-name-tooltip").first();
  await expect(tooltip).toHaveCSS("opacity", "1");
  await expect(tooltip).toBeVisible();
});
