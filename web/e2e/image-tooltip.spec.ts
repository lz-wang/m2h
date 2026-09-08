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
  // The images load lazily, and a still-pending placeholder keeps its
  // tooltip out of layout (display: none) — every geometry assertion below
  // reads the loaded presentation, so wait for the lazy machines to settle.
  await page.waitForFunction(
    (count) =>
      document.querySelectorAll<HTMLImageElement>(
        '.markdown-body img[data-m2h-lazy-state="loaded"]',
      ).length === count,
    frameCount,
  );
}

interface TooltipGeometry {
  imageWidth: number;
  tooltipWidth: number;
}

// The tooltip's width cap as the reader experiences it: 80% of the visible
// page (never wider than it minus the side gutters). Read live, so the
// assertions keep expressing the product contract if the viewport or the
// formula changes.
async function readTooltipCap(page: Page): Promise<number> {
  return page.evaluate(() =>
    Math.min(window.innerWidth * 0.8, window.innerWidth - 32),
  );
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
    const cap = await readTooltipCap(page);

    for (let index = 0; index < frameCount; index += 1) {
      const geometry = await measureTooltip(page, index);
      // The label's width is decided by its text (up to the cap), never
      // squeezed to the icon-sized image it floats over.
      expect(geometry.tooltipWidth).toBeGreaterThan(geometry.imageWidth);
      // Viewport cap; the long middle name reaches it, the short names stay
      // below, so this also pins the ellipsis ceiling.
      expect(geometry.tooltipWidth).toBeLessThanOrEqual(cap);
    }

    await expectNoHorizontalOverflow(page);
  });
}

test("caps the long name at 80% of the viewport while the short name stays under it", async ({
  page,
}) => {
  await openDocument(page);
  const cap = await readTooltipCap(page);

  const shortName = await measureTooltip(page, 0);
  const longName = await measureTooltip(page, 1);

  expect(shortName.tooltipWidth).toBeGreaterThan(16);
  expect(shortName.tooltipWidth).toBeLessThan(cap);
  // The long fixture name far exceeds 80% of the viewport's worth of text,
  // so the cap — not the text — decides its width.
  expect(longName.tooltipWidth).toBeCloseTo(cap, 0);
});

test("reveals the tooltip on frame hover", async ({ page }) => {
  await openDocument(page);

  const frame = page.locator(".m2h-image-frame").first();
  await frame.hover();

  const tooltip = page.locator(".m2h-image-name-tooltip").first();
  await expect(tooltip).toHaveCSS("opacity", "1");
  await expect(tooltip).toBeVisible();
});

test("shows intrinsic size and format in the metadata row", async ({
  page,
}) => {
  await openDocument(page);

  // The metadata part reports the image's own pixels and its source format,
  // independent of how small the page renders it. With an alt present it is
  // wrapped in parentheses after the name.
  for (const tooltip of await page.locator(".m2h-image-name-tooltip").all()) {
    await expect(tooltip.locator(".m2h-image-tooltip-meta")).toHaveText(
      "(16 × 16, PNG)",
    );
  }
});

test("reads the tooltip as one line of name plus metadata", async ({
  page,
}) => {
  await openDocument(page);

  const tooltip = page.locator(".m2h-image-name-tooltip").first();

  // The user-facing contract: the whole label reads "name (size, format)".
  // The flex gap renders as visual space without a text node between the
  // spans, so the reading-order match tolerates the missing literal space.
  await expect(tooltip).toContainText(/短名称\s*\(16 × 16, PNG\)/);

  // Text assertions cannot catch a flex-direction: column regression, so the
  // two spans' real boxes are compared: one line means one top edge.
  const layout = await tooltip.evaluate((element) => {
    const alt = element.querySelector(".m2h-image-tooltip-alt");
    const meta = element.querySelector(".m2h-image-tooltip-meta");

    if (!(alt instanceof HTMLElement) || !(meta instanceof HTMLElement)) {
      throw new Error("tooltip parts missing");
    }

    return {
      altTop: alt.getBoundingClientRect().top,
      metaTop: meta.getBoundingClientRect().top,
    };
  });
  expect(Math.abs(layout.altTop - layout.metaTop)).toBeLessThanOrEqual(1);
});
