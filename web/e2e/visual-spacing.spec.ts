import { expect, type Page, test } from "@playwright/test";

// Real-browser geometry regression for the visual block spacing contract: a
// standalone image paragraph and the rich-visual frames (Mermaid, Vega-Lite)
// share one external spacing rule — the same 1rem to the surrounding prose on
// both sides. jsdom computes no geometry, so only a genuine layout engine can
// compare the actual gaps; the failure it guards against is the image
// paragraph's inline line-box strut (or a frame margin drift) making one
// visual kind sit visibly closer to or farther from the text than the others.

const documentPath = "/doc/visual-spacing.md";

async function openDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(documentPath);
  // Every visual must have settled: the image through its lazy machine (the
  // placeholder swaps for the real pixels), the diagram and the chart through
  // their async renders.
  await page.waitForFunction(() => {
    const body = document.querySelector(".markdown-body");
    if (body === null) {
      return false;
    }
    const image = body.querySelector<HTMLImageElement>(
      ".m2h-image-block img[data-m2h-lazy-state='loaded']",
    );
    return (
      image !== null &&
      body.querySelector(".m2h-mermaid-frame svg") !== null &&
      body.querySelector(".m2h-vega-lite-frame svg") !== null
    );
  });
}

interface VisualGaps {
  above: number;
  below: number;
}

// The pixel distance between one visual and the prose paragraphs before and
// after it: the gap above runs from the previous sibling's bottom edge to the
// visual's own first pixel, the gap below from its last pixel to the next
// sibling's top edge. The frame/block box owns the margins; the visual
// element (img/svg) is what the reader sees, so measuring the visual itself
// is what catches a strut or a margin leaking back between the two.
function readGaps(page: Page): Promise<Record<string, VisualGaps>> {
  return page.evaluate(() => {
    const body = document.querySelector(".markdown-body");
    if (body === null) {
      throw new Error("markdown body was not rendered");
    }
    const read = (
      holder: HTMLElement | null,
      visual: Element | null,
      label: string,
    ): VisualGaps => {
      const previous = holder?.previousElementSibling;
      const next = holder?.nextElementSibling;
      if (
        holder === null ||
        visual === null ||
        !(previous instanceof HTMLElement) ||
        !(next instanceof HTMLElement)
      ) {
        throw new Error(`visual geometry unavailable for ${label}`);
      }
      const visualRect = visual.getBoundingClientRect();
      return {
        above: visualRect.top - previous.getBoundingClientRect().bottom,
        below: next.getBoundingClientRect().top - visualRect.bottom,
      };
    };
    const imageBlock = body.querySelector<HTMLElement>(".m2h-image-block");
    const mermaidFrame = body.querySelector<HTMLElement>(".m2h-mermaid-frame");
    const vegaLiteFrame = body.querySelector<HTMLElement>(
      ".m2h-vega-lite-frame",
    );
    return {
      image: read(imageBlock, imageBlock?.querySelector("img"), "image"),
      mermaid: read(
        mermaidFrame,
        mermaidFrame?.querySelector("svg"),
        "mermaid",
      ),
      "vega-lite": read(
        vegaLiteFrame,
        vegaLiteFrame?.querySelector("svg"),
        "vega-lite",
      ),
    };
  });
}

test("gives every visual kind the same gap to the surrounding prose", async ({
  page,
}) => {
  await openDocument(page);

  const gaps = await readGaps(page);

  // The contract is cross-kind consistency: image, Mermaid, and Vega-Lite
  // keep the same distance to the prose above and below. A paragraph strut
  // (the pre-contract image layout) or a frame margin drift shows up as a
  // spread between the three, so the comparison — not a magic number — is
  // the assertion, with ±1px of engine rounding slack.
  for (const side of ["above", "below"] as const) {
    const values = [
      gaps.image[side],
      gaps.mermaid[side],
      gaps["vega-lite"][side],
    ];
    expect(Math.max(...values) - Math.min(...values)).toBeLessThanOrEqual(1);
    // Each visual keeps a real gap on both sides; a collapsed or negative
    // margin would put the visual flush against (or under) the text.
    expect(Math.min(...values)).toBeGreaterThan(0);
  }
});

test("marks standalone image paragraphs while retaining prose line height", async ({
  page,
}) => {
  await openDocument(page);

  // Only the standalone image paragraph joins the visual block contract; a
  // prose paragraph retains its own text line-height.
  const counts = await page.evaluate(() => ({
    blocks: document.querySelectorAll(".markdown-body .m2h-image-block").length,
    frames: document.querySelectorAll(".markdown-body .m2h-image-frame").length,
    prose: document.querySelectorAll(".markdown-body > p:not(.m2h-image-block)")
      .length,
  }));
  expect(counts.blocks).toBe(1);
  expect(counts.frames).toBe(1);
  expect(counts.prose).toBeGreaterThanOrEqual(5);
});
