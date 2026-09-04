import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for the image Lightbox. The behaviors that matter
// here are exactly the ones jsdom cannot compute: the modal dialog's scroll
// lock, the portaled overlay's layout, and the two hard invariants of the
// feature — through every lightbox interaction (open, next/previous, zoom,
// rotate, pan, close) the window's scrollY and the URL hash must not move, and
// the reading position must be exactly where it was left.

const documentPath = "/doc/image-lightbox.md";

// Wait until the article has painted and the lightbox triggers are injected,
// then expose the toolbar/counter helpers every test below shares.
async function openDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(documentPath);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body img") !== null &&
      document.querySelectorAll(".m2h-lightbox-trigger").length >= 3,
  );
}

// Bring the first trigger into view explicitly, then let the heading spy
// settle so the recorded hash is the stable pre-lightbox state. Returning the
// invariants this way means no later Playwright auto-scroll can pollute them.
async function captureInvariants(page: Page) {
  await page.locator(".m2h-lightbox-trigger").first().scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  return {
    scrollY: await page.evaluate(() => window.scrollY),
    hash: await page.evaluate(() => location.hash),
  };
}

async function expectInvariantsUnchanged(
  page: Page,
  before: { scrollY: number; hash: string },
) {
  const afterScrollY = await page.evaluate(() => window.scrollY);
  const afterHash = await page.evaluate(() => location.hash);
  expect(Math.abs(afterScrollY - before.scrollY)).toBeLessThanOrEqual(1);
  expect(afterHash).toBe(before.hash);
}

async function openLightbox(page: Page, triggerIndex = 0) {
  // The trigger is hover-gated in the reader (opacity/pointer-events until
  // its frame is hovered). Hovering the FRAME first — always hoverable —
  // makes the trigger clickable; the mouse is then already inside the frame,
  // exactly how a user reaches the magnifier.
  await page.locator(".m2h-image-frame").nth(triggerIndex).hover();
  await page.locator(".m2h-lightbox-trigger").nth(triggerIndex).click();
  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  return popup;
}

// The pan component of the current transform, parsed out of the inline style.
async function imagePan(page: Page): Promise<{ x: number; y: number }> {
  return page.evaluate(() => {
    const image = document.querySelector<HTMLImageElement>(
      ".image-lightbox-image",
    );
    if (image === null) {
      throw new Error("lightbox image was not rendered");
    }
    const match = image.style.transform.match(
      /translate3d\((-?[\d.]+)px, (-?[\d.]+)px, 0px\)/,
    );
    if (match === null) {
      throw new Error(`unexpected transform: ${image.style.transform}`);
    }
    return { x: Number(match[1]), y: Number(match[2]) };
  });
}

// Wait until the lightbox image has been laid out at its fitted size: the
// geometry the rotate fit and the pan clamp consume is reported by the
// ResizeObserver only once the element has a box.
async function waitForFittedImage(
  page: Page,
  min: number,
): Promise<{ x: number; y: number; width: number; height: number }> {
  const image = page.locator(".image-lightbox-image");
  await expect
    .poll(async () => (await image.boundingBox())?.height ?? 0, {
      timeout: 5_000,
    })
    .toBeGreaterThanOrEqual(min);
  const box = await image.boundingBox();
  if (box === null) {
    throw new Error("lightbox image was not rendered");
  }
  return box;
}

test("keeps scrollY and hash invariant across a full lightbox session", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  const displayed = page.locator(".image-lightbox-image");
  const counter = page.locator(
    '.image-lightbox-counter > span[aria-hidden="true"]',
  );
  await expect(counter).toHaveText("1 / 3");
  // The fixtures are three different files, so the counter and the actually
  // displayed image must switch together — an index-only regression (the
  // stale-index table-sort bug) shows the wrong file here.
  await expect(displayed).toHaveAttribute("src", /landscape\.png$/);

  // Navigate, rotate, zoom — the whole toolbar round.
  await page.getByRole("button", { name: "下一项" }).click();
  await expect(counter).toHaveText("2 / 3");
  await expect(displayed).toHaveAttribute("src", /portrait\.png$/);
  // The portrait fixture (480×1200) fits the stage height-first.
  const portraitBox = await waitForFittedImage(page, 700);
  expect(portraitBox.height).toBeGreaterThan(portraitBox.width);
  await page.getByRole("button", { name: "下一项" }).click();
  await expect(counter).toHaveText("3 / 3");
  await expect(displayed).toHaveAttribute("src", /square\.png$/);
  await page.getByRole("button", { name: "顺时针旋转" }).click();
  await page.getByRole("button", { name: "放大图片" }).click();

  // The transform carries the rotated, zoomed state on the compositor.
  const transform = await page.evaluate(() => {
    const image = document.querySelector<HTMLImageElement>(
      ".image-lightbox-image",
    );
    if (image === null) {
      throw new Error("lightbox image was not rendered");
    }
    return image.style.transform;
  });
  expect(transform).toContain("rotate(90deg)");
  expect(transform).toContain("scale(");
  expect(transform).not.toContain("scale(1)");

  // Close through the X button; the lightbox is gone and nothing moved.
  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("closes through Escape without moving the document", async ({ page }) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  await page.getByRole("button", { name: "下一项" }).click();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("2 / 3");

  await page.keyboard.press("Escape");
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("closes through a blank-area press without moving the document", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  await page.getByRole("button", { name: "下一项" }).click();

  // A press on the overlay's top-left corner: empty scrim, away from both the
  // centered image and the top-right close button.
  await page.mouse.click(12, 12);
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("does not close on presses over the image or the toolbar", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  // Press the toolbar (its zoom button), then the image itself.
  await page.getByRole("button", { name: "放大图片" }).click();
  await page.locator(".image-lightbox-image").click();
  await expect(page.locator(".image-lightbox")).toBeVisible();
  await expectInvariantsUnchanged(page, before);
});

test("disables previous on the first image and next on the last", async ({
  page,
}) => {
  await openDocument(page);
  await captureInvariants(page);

  await openLightbox(page, 0);
  const previous = page.getByRole("button", { name: "上一项" });
  await expect(previous).toBeDisabled();

  const next = page.getByRole("button", { name: "下一项" });
  await next.click();
  await next.click();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("3 / 3");
  await expect(next).toBeDisabled();
  await expect(previous).toBeEnabled();
});

test("keeps a rotated landscape image clear of the bottom toolbar", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  // The first image is the 1200×600 landscape fixture: tall enough that a
  // quarter turn produces a portrait taller than the toolbar-free area, so a
  // fit computed against the whole viewport would slide under the toolbar.
  await openLightbox(page, 0);
  const toolbar = page.locator(".image-lightbox-toolbar");

  // Unrotated fit: the image already sits inside the stage.
  let imageBox = await waitForFittedImage(page, 599);
  let toolbarBox = await toolbar.boundingBox();
  expect(toolbarBox).not.toBeNull();
  expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(toolbarBox.y);

  // A quarter turn swaps the visual axes; the re-fit must land inside the
  // stage (the popup minus the toolbar reserve), never under the toolbar.
  await page.getByRole("button", { name: "顺时针旋转" }).click();
  imageBox = await waitForFittedImage(page, 700);
  toolbarBox = await toolbar.boundingBox();
  expect(imageBox.y + imageBox.height).toBeLessThanOrEqual(toolbarBox.y + 1);

  await expectInvariantsUnchanged(page, before);
});

test("keeps the close control above a maximally zoomed image", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);

  // Zoom to the 5x cap: the transformed image paints far past its layout
  // box and covers the whole viewport — including the close button's spot.
  const zoomIn = page.getByRole("button", { name: "放大图片" });
  while (await zoomIn.isEnabled()) {
    await zoomIn.click();
  }

  const close = page.getByRole("button", { name: "关闭视觉内容预览" });
  const box = await close.boundingBox();
  if (box === null) {
    throw new Error("close button was not rendered");
  }

  // elementFromPoint verifies what the user actually experiences: the close
  // button must be the topmost hit target at its own center, not merely carry
  // a higher z-index — a zoomed image painting (or hit-testing) over it fails
  // here even if the styles look right.
  const closeIsTopmost = await page.evaluate(
    ({ x, y }) => {
      const element = document.elementFromPoint(x, y);
      return element?.closest(".image-lightbox-close") !== null;
    },
    {
      x: box.x + box.width / 2,
      y: box.y + box.height / 2,
    },
  );

  expect(closeIsTopmost).toBe(true);

  // The button is not just visible above the image — it still closes.
  await close.click();
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("clamps pointer pans to the fitted stage after zooming", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  const start = await waitForFittedImage(page, 599);

  // Zoom to the 5x cap: the 1200×600 landscape at 1280×900 sits at scale 5,
  // so the drag bounds are maxPanX = (1200·5 − 1248)/2 = 2376 and maxPanY =
  // (600·5 − 788)/2 = 1106.
  const zoomIn = page.getByRole("button", { name: "放大图片" });
  for (let click = 0; click < 8; click += 1) {
    await zoomIn.click();
  }
  await expect(zoomIn).toBeDisabled();

  // Drag far past every edge; the pan must stop at the fitted bound instead
  // of following the pointer.
  await page.mouse.move(start.x + start.width / 2, start.y + start.height / 2);
  await page.mouse.down();
  await page.mouse.move(
    start.x + start.width / 2 - 5000,
    start.y + start.height / 2 - 5000,
    { steps: 10 },
  );
  await page.mouse.up();

  const pan = await imagePan(page);
  expect(pan.x).toBeGreaterThanOrEqual(-2377);
  expect(pan.x).toBeLessThanOrEqual(-2375);
  expect(pan.y).toBeGreaterThanOrEqual(-1107);
  expect(pan.y).toBeLessThanOrEqual(-1105);

  await expectInvariantsUnchanged(page, before);
});

test("keeps the linked image's anchor while its trigger opens the lightbox", async ({
  page,
}) => {
  await openDocument(page);

  // Pressing the linked image itself follows the anchor: the fragment moves
  // and no lightbox appears.
  await page.locator(".m2h-image-frame a img").scrollIntoViewIfNeeded();
  await page.locator(".m2h-image-frame a img").click();
  await page.waitForFunction(() => decodeURIComponent(location.hash) !== "");
  expect(decodeURIComponent(await page.evaluate(() => location.hash))).toBe(
    "#目标章节",
  );
  expect(await page.locator(".image-lightbox").count()).toBe(0);

  // The magnifier on the same frame opens the lightbox instead. Hover the
  // frame first: the trigger is hover-gated in the reader.
  await page.locator(".m2h-image-frame:has(> a)").hover();
  await page.locator(".m2h-image-frame:has(> a) .m2h-lightbox-trigger").click();
  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  await expect(
    page.locator('.image-lightbox-counter > span[aria-hidden="true"]'),
  ).toHaveText("2 / 3");
});

// --- Mermaid diagrams in the shared lightbox --------------------------------
//
// The mermaid fixture interleaves image → diagram → image, so these tests
// verify the two visual kinds really browse as ONE sequence with the shared
// toolbar, and that an inline diagram snapshot survives the same zoom /
// rotate / pan / close round a plain image goes through.

const mermaidDocumentPath = "/doc/mermaid-lightbox.md";

async function openMermaidDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(mermaidDocumentPath);
  // The diagram must finish rendering — its SVG lands inside the frame —
  // before the trigger has anything to open.
  await page.waitForFunction(
    () =>
      document.querySelector(".m2h-mermaid-frame svg") !== null &&
      document.querySelectorAll(".m2h-lightbox-trigger").length >= 3,
  );
}

async function openMermaidLightbox(page: Page) {
  await page.locator(".m2h-mermaid-frame").hover();
  await page.locator(".m2h-mermaid-frame .m2h-lightbox-trigger").click();
  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  return popup;
}

// Same contract as captureInvariants, but anchored at the diagram: hovering
// the mermaid frame (below the first image in this fixture) auto-scrolls to
// it, so the invariants must be recorded from the diagram's position, not
// the first trigger's.
async function captureMermaidInvariants(page: Page) {
  await page.locator(".m2h-mermaid-frame").scrollIntoViewIfNeeded();
  await page.waitForTimeout(300);
  return {
    scrollY: await page.evaluate(() => window.scrollY),
    hash: await page.evaluate(() => location.hash),
  };
}

// A screen point inside the diagram that hits no selectable text: by contract
// a mouse press on diagram text is a selection, so pan gestures must
// originate on shapes or the diagram background.
async function findNonTextPoint(page: Page): Promise<{ x: number; y: number }> {
  return page.evaluate(() => {
    const svg = document.querySelector(".image-lightbox-vector > svg");
    if (svg === null) {
      throw new Error("lightbox svg was not rendered");
    }
    const rect = svg.getBoundingClientRect();
    for (let step = 0; step < 400; step += 1) {
      const x = rect.left + (rect.width * (step % 20)) / 19;
      const y = rect.top + (rect.height * Math.floor(step / 20)) / 19;
      // elementFromPoint respects the stage's clipping: a hit outside the
      // diagram lands on the popup scrim, whose press closes the Lightbox —
      // so the point must both hit-test into the svg and carry no text.
      const hit = document.elementFromPoint(x, y);
      if (
        hit !== null &&
        svg.contains(hit) &&
        hit.closest("text, tspan, foreignObject") === null
      ) {
        return { x, y };
      }
    }
    throw new Error("no non-text point found in the lightbox svg");
  });
}

// A mouse drag across a diagram label. Mermaid emits labels either as SVG
// `<text>` or as HTML inside a `<foreignObject>` (htmlLabels); both are
// selectable and the locator accepts either shape. Edge labels carry empty
// zero-size boxes, so only labels with real text qualify.
async function dragAcrossDiagramLabel(page: Page): Promise<string> {
  const label = page
    .locator(".image-lightbox-vector text", { hasText: /\S/ })
    .or(
      page
        .locator(".image-lightbox-vector foreignObject div", {
          hasText: /\S/,
        })
        .first(),
    )
    .first();
  await expect
    .poll(async () => (await label.boundingBox())?.width ?? 0, {
      timeout: 5_000,
    })
    .toBeGreaterThan(0);
  const box = await label.boundingBox();
  if (box === null) {
    throw new Error("diagram label was not rendered");
  }
  const y = box.y + box.height / 2;
  await page.mouse.move(box.x + box.width * 0.15, y);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width * 0.9, y, { steps: 8 });
  await page.mouse.up();
  return page.evaluate(() => window.getSelection()?.toString() ?? "");
}

test("opens a mermaid diagram inside the shared image sequence", async ({
  page,
}) => {
  await openMermaidDocument(page);
  const before = await captureMermaidInvariants(page);

  await openMermaidLightbox(page);
  const displayed = page.locator(".image-lightbox-vector > svg");
  const counter = page.locator(
    '.image-lightbox-counter > span[aria-hidden="true"]',
  );
  // The diagram is the middle item of the image → diagram → image document.
  await expect(counter).toHaveText("2 / 3");
  await expect(displayed).toHaveCount(1);

  // Both neighbors are real images; the sequence is shared, not split into
  // per-kind galleries.
  await page.getByRole("button", { name: "下一项" }).click();
  await expect(counter).toHaveText("3 / 3");
  await expect(page.locator(".image-lightbox-image")).toHaveAttribute(
    "src",
    /square\.png$/,
  );
  await page.getByRole("button", { name: "上一项" }).click();
  await page.getByRole("button", { name: "上一项" }).click();
  await expect(counter).toHaveText("1 / 3");
  await expect(page.locator(".image-lightbox-image")).toHaveAttribute(
    "src",
    /landscape\.png$/,
  );

  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

test("namespaces Mermaid SVG identifiers in its Lightbox snapshot", async ({
  page,
}) => {
  await openMermaidDocument(page);
  const sourceIDs = await page
    .locator(".m2h-mermaid-frame svg [id]")
    .evaluateAll((elements) => elements.map((element) => element.id));
  expect(sourceIDs.length).toBeGreaterThan(0);

  await openMermaidLightbox(page);
  const lightboxIDs = await page
    .locator(".image-lightbox-vector svg [id]")
    .evaluateAll((elements) => elements.map((element) => element.id));
  expect(lightboxIDs.length).toBeGreaterThan(0);
  expect(lightboxIDs.every((id) => id.startsWith("m2h-lightbox-"))).toBe(true);
  expect(lightboxIDs.some((id) => sourceIDs.includes(id))).toBe(false);
});

// Renaming the snapshot's root id must carry the renderer's scoped CSS with
// it: Mermaid keys its palette rules on the root id (`#m2h-mermaid-N { … }`),
// and a missed selector silently strips the diagram's font and colors in the
// Lightbox. The computed styles of the body diagram and its snapshot must
// therefore agree exactly.
test("keeps the snapshot's scoped styles equivalent to the body diagram", async ({
  page,
}) => {
  await openMermaidDocument(page);
  await openMermaidLightbox(page);
  await expect(page.locator(".image-lightbox-vector > svg")).toHaveCount(1);

  const styles = await page.evaluate(() => {
    const read = (svg: Element | null) => {
      if (svg === null) {
        throw new Error("svg was not rendered");
      }
      const root = getComputedStyle(svg);
      const shape = svg.querySelector("path, rect, polygon, circle, line");
      const shapeStyle =
        shape === null ? null : getComputedStyle(shape as Element);
      return {
        fontFamily: root.fontFamily,
        fill: root.fill,
        shapeFill: shapeStyle?.fill ?? "",
        shapeStroke: shapeStyle?.stroke ?? "",
      };
    };
    return {
      body: read(document.querySelector(".m2h-mermaid-frame svg")),
      lightbox: read(document.querySelector(".image-lightbox-vector > svg")),
    };
  });

  expect(styles.lightbox.fontFamily).toBe(styles.body.fontFamily);
  expect(styles.lightbox.fontFamily).not.toBe("");
  expect(styles.lightbox.fill).toBe(styles.body.fill);
  expect(styles.lightbox.shapeFill).toBe(styles.body.shapeFill);
  expect(styles.lightbox.shapeStroke).toBe(styles.body.shapeStroke);
});

// Playwright derives an element's box from Chromium content quads, which can
// transiently come back empty while the stage's enter transition is mid
// transform-animation; a settled poll reads the real geometry. Measuring the
// root <svg> itself is the point of these tests: a wrapper that grows around
// a diagram pinned by Mermaid's own inline max-width (width="100%" +
// `max-width: <natural>px`) is exactly the regression that wrapper-level
// assertions cannot see.
async function waitForVectorSvgBox(page: Page) {
  const svg = page.locator(".image-lightbox-vector > svg");
  await expect
    .poll(async () => (await svg.boundingBox())?.width ?? 0, {
      timeout: 5_000,
    })
    .toBeGreaterThan(0);
  const box = await svg.boundingBox();
  if (box === null) {
    throw new Error("lightbox svg was not rendered");
  }
  return box;
}

test("zooms the real mermaid svg around the stage center", async ({ page }) => {
  await openMermaidDocument(page);
  await openMermaidLightbox(page);
  const svg = page.locator(".image-lightbox-vector > svg");
  await expect(svg).toHaveCount(1);

  const before = await waitForVectorSvgBox(page);
  await page.getByRole("button", { name: "放大图片" }).click();
  const after = await waitForVectorSvgBox(page);

  // The root <svg> itself grows by the zoom step (not merely the wrapper)…
  expect(after.width / before.width).toBeCloseTo(1.25, 1);
  expect(after.height / before.height).toBeCloseTo(1.25, 1);
  // …and with pan/rotation at rest the growth stays centered — a wrapper-only
  // regression shows up as sideways drift instead.
  expect(
    Math.abs(after.x + after.width / 2 - (before.x + before.width / 2)),
  ).toBeLessThanOrEqual(1);
  expect(
    Math.abs(after.y + after.height / 2 - (before.y + before.height / 2)),
  ).toBeLessThanOrEqual(1);
});

// The wheel listener is attached directly on the pan wrapper (React's root
// wheel listener is passive), so only a real browser wheel event proves the
// gesture zooms while being claimed away from native scrolling — including a
// document locked under the modal overlay.
test("zooms a mermaid diagram with a real wheel gesture without scrolling", async ({
  page,
}) => {
  await openMermaidDocument(page);
  await openMermaidLightbox(page);
  const svg = page.locator(".image-lightbox-vector > svg");
  await expect(svg).toHaveCount(1);

  // The wheel listener lives on the pan wrapper and svg events bubble to it,
  // so hovering the wrapper (not any single text or shape inside) places the
  // pointer wherever the wheel that follows is guaranteed to be claimed.
  await page.locator(".image-lightbox-vector").hover();
  const before = await waitForVectorSvgBox(page);
  const scrollBefore = await page.evaluate(() => ({
    x: window.scrollX,
    y: window.scrollY,
  }));

  await page.mouse.wheel(0, -120);
  await page.mouse.wheel(0, -120);

  const after = await waitForVectorSvgBox(page);
  expect(after.width).toBeGreaterThan(before.width);
  expect(after.height).toBeGreaterThan(before.height);
  expect(await page.evaluate(() => window.scrollX)).toBe(scrollBefore.x);
  expect(await page.evaluate(() => window.scrollY)).toBe(scrollBefore.y);
});

// Gesture arbitration on vector visuals: diagram text is real selectable
// content (Mermaid labels are `<text>` or HTML in a `<foreignObject>`), so a
// mouse drag across a label must produce a native selection — not a pan —
// even when zoomed far enough that pan room exists.
test("selects mermaid diagram text without panning", async ({ page }) => {
  await openMermaidDocument(page);
  await openMermaidLightbox(page);
  await expect(page.locator(".image-lightbox-vector > svg")).toHaveCount(1);

  // Zoom first: only then is there pan room, making the arbitration real.
  const zoomIn = page.getByRole("button", { name: "放大图片" });
  await zoomIn.click();
  await zoomIn.click();

  const selected = await dragAcrossDiagramLabel(page);
  expect(selected.trim().length).toBeGreaterThan(0);

  const pan = await page.evaluate(() => {
    const visual = document.querySelector<HTMLElement>(
      ".image-lightbox-vector-transform",
    );
    return visual?.style.transform ?? "";
  });
  // Chromium normalizes the unitless z to 0px; a pan would show non-zero x/y.
  expect(pan).toContain("translate3d(0px, 0px, 0px)");
});

test("rotates, zooms, pans and closes a mermaid diagram without moving the document", async ({
  page,
}) => {
  await openMermaidDocument(page);
  const before = await captureMermaidInvariants(page);

  await openMermaidLightbox(page);
  await expect(page.locator(".image-lightbox-vector > svg")).toHaveCount(1);

  // Rotate: the transform carries the quarter turn like any image.
  await page.getByRole("button", { name: "顺时针旋转" }).click();
  await page.waitForFunction(() => {
    const visual = document.querySelector<HTMLElement>(
      ".image-lightbox-vector-transform",
    );
    return visual?.style.transform.includes("rotate(90deg)") ?? false;
  });

  // Zoom to the 5x cap, then drag far past every edge from a point that hits
  // no selectable text — a press on diagram text is a selection by contract.
  // The pan must stop at the exact rotated-stage boundary, calculated from
  // the dimensions the Lightbox itself uses rather than an arbitrary bound.
  const zoomIn = page.getByRole("button", { name: "放大图片" });
  while (await zoomIn.isEnabled()) {
    await zoomIn.click();
  }
  const origin = await findNonTextPoint(page);
  await page.mouse.move(origin.x, origin.y);
  await page.mouse.down();
  await page.mouse.move(origin.x - 5000, origin.y - 5000, { steps: 10 });
  await page.mouse.up();

  const { pan, geometry } = await page.evaluate(() => {
    const transform = document.querySelector<HTMLElement>(
      ".image-lightbox-vector-transform",
    );
    const vector = document.querySelector<HTMLElement>(
      ".image-lightbox-vector",
    );
    const stage = document.querySelector<HTMLElement>(".image-lightbox-stage");
    if (transform === null || vector === null || stage === null) {
      throw new Error("lightbox geometry was not rendered");
    }
    const match = transform.style.transform.match(
      /translate3d\((-?[\d.]+)px, (-?[\d.]+)px, 0px\)/,
    );
    const vectorStyle = getComputedStyle(vector);
    const stageRect = stage.getBoundingClientRect();
    return {
      pan: { x: Number(match?.[1] ?? 0), y: Number(match?.[2] ?? 0) },
      geometry: {
        stageWidth: stageRect.width,
        stageHeight: stageRect.height,
        renderedWidth: Number.parseFloat(vectorStyle.width),
        renderedHeight: Number.parseFloat(vectorStyle.height),
      },
    };
  });
  const maxPanX = Math.max(
    0,
    (geometry.renderedHeight - geometry.stageWidth) / 2,
  );
  const maxPanY = Math.max(
    0,
    (geometry.renderedWidth - geometry.stageHeight) / 2,
  );
  expect(pan.x).toBeCloseTo(-maxPanX, 1);
  expect(pan.y).toBeCloseTo(-maxPanY, 1);

  // Still closable at max zoom: the close button stays on top and works.
  const close = page.getByRole("button", { name: "关闭视觉内容预览" });
  const closeBox = await close.boundingBox();
  if (closeBox === null) {
    throw new Error("close button was not rendered");
  }
  const closeIsTopmost = await page.evaluate(
    ({ x, y }) => {
      const element = document.elementFromPoint(x, y);
      return element?.closest(".image-lightbox-close") !== null;
    },
    {
      x: closeBox.x + closeBox.width / 2,
      y: closeBox.y + closeBox.height / 2,
    },
  );
  expect(closeIsTopmost).toBe(true);
  await close.click();
  await expect(page.locator(".image-lightbox")).toBeHidden();
  await expectInvariantsUnchanged(page, before);
});

// The diagram lightbox is an inline vector snapshot on a theme-aware canvas:
// its markup stays byte-identical across the whole zoom range — no
// re-serialization or rasterization — while the stage behind it renders white
// in the light theme and black in the dark one. Bitmap images keep the
// transparent stage; the modal backdrop is unchanged.
test("renders the mermaid lightbox as a vector snapshot on a theme-aware canvas", async ({
  page,
}) => {
  await openMermaidDocument(page);
  await openMermaidLightbox(page);
  await expect(page.locator(".image-lightbox-vector > svg")).toHaveCount(1);

  const readLightbox = () =>
    page.evaluate(() => {
      const stage = document.querySelector<HTMLElement>(
        ".image-lightbox-stage",
      );
      const svg = document.querySelector(".image-lightbox-vector > svg");
      if (stage === null || svg === null) {
        throw new Error("lightbox stage was not rendered");
      }
      return {
        kind: stage.dataset.visualKind,
        background: getComputedStyle(stage).backgroundColor,
        markup: svg.outerHTML,
      };
    });

  // Light theme: the diagram canvas is white and the snapshot stays inline.
  const light = await readLightbox();
  expect(light.kind).toBe("mermaid");
  expect(light.background).toBe("rgb(255, 255, 255)");
  expect(light.markup).toContain("<svg");

  // Zoom from 1x to the 5x cap without serializing or rasterizing the SVG.
  const zoomIn = page.getByRole("button", { name: "放大图片" });
  while (await zoomIn.isEnabled()) {
    await zoomIn.click();
  }
  expect((await readLightbox()).markup).toBe(light.markup);
  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(page.locator(".image-lightbox")).toBeHidden();

  // Dark theme: a fresh (dark-palette) snapshot, black canvas.
  await page.goto(`${mermaidDocumentPath}?mode=dark`);
  await page.waitForFunction(
    () =>
      document.documentElement.classList.contains("dark") &&
      document.querySelector(".m2h-mermaid-frame svg") !== null,
  );
  await page.locator(".m2h-mermaid-frame").hover();
  await page.locator(".m2h-mermaid-frame .m2h-lightbox-trigger").click();
  const dark = await readLightbox();
  expect(dark.background).toBe("rgb(0, 0, 0)");
  expect(dark.markup).toContain("<svg");
});

// --- Diagrams whose render never produced an SVG -----------------------------
//// Lightbox availability is a function of the SVG's real presence, not of the
// frame's existence: an invalid diagram keeps its frame (and source text) but
// must offer no magnifier — neither clickable nor keyboard-reachable — while
// a valid diagram in the same document behaves exactly as before.

const invalidMermaidDocumentPath = "/doc/mermaid-invalid.md";

async function openInvalidMermaidDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(invalidMermaidDocumentPath);
  // Both frames are created synchronously; the paints run in document order,
  // so once the valid (second) diagram's trigger unhides, the invalid first
  // diagram's paint attempt has settled too.
  await page.waitForFunction(
    () =>
      document.querySelectorAll(".m2h-mermaid-frame").length === 2 &&
      // Scope to the container: the trigger's magnifier icon is an SVG too.
      document.querySelectorAll(".m2h-mermaid-frame > .mermaid svg").length ===
        1,
  );
  await page.waitForFunction(() => {
    const frames = document.querySelectorAll<HTMLElement>(".m2h-mermaid-frame");
    const valid = frames[1];
    return valid
      ? valid.querySelector<HTMLButtonElement>(":scope > .m2h-lightbox-trigger")
          ?.hidden === false
      : false;
  });
}

test("hides the magnifier of a diagram that never rendered", async ({
  page,
}) => {
  await openInvalidMermaidDocument(page);

  const invalid = page.locator(".m2h-mermaid-frame").first();
  const valid = page.locator(".m2h-mermaid-frame").nth(1);

  // The invalid container collapsed into the shared failure placeholder (the
  // raw Mermaid source never shows) and never gained an SVG, the lightbox
  // marker, or an operable trigger. (svg assertions scope to the container:
  // the trigger's magnifier icon is an SVG too.)
  await expect(invalid.locator(".mermaid svg")).toHaveCount(0);
  await expect(invalid.locator(".m2h-rich-visual-error-title")).toHaveText(
    "Mermaid 图表渲染失败",
  );
  await expect(invalid.locator('[data-m2h-lightbox-item="true"]')).toHaveCount(
    0,
  );
  const trigger = invalid.locator(".m2h-lightbox-trigger");
  await expect(trigger).toBeHidden();
  // The hidden attribute must actually apply: the trigger's own display rule
  // (inline-flex) would otherwise override the UA's [hidden] style and leave
  // an invisible-but-clickable dead button.
  await expect(trigger).toHaveCSS("display", "none");

  // The valid diagram in the same document is unaffected.
  await expect(valid.locator('[data-m2h-lightbox-item="true"]')).toHaveCount(1);
  await expect(valid.locator(".m2h-lightbox-trigger")).toBeVisible();
  await expect(valid.locator(".mermaid svg")).toHaveCount(1);
});

// --- Enter / exit transitions ------------------------------------------------
//
// The popup's lifecycle is split: closing only starts the exit transition
// (data-ending-style) and the snapshot state — with the popup — is dropped
// once Base UI reports the animation finished. These tests sample every
// animation frame from before the action, so a regression that unmounts the
// popup immediately (or never enters the transition states) fails here.

// Sample each frame until the element exists without data-starting-style;
// resolves whether the starting state was ever observed.
function observeEnterStyle(page: Page, selector: string) {
  return page.evaluate(
    ({ selector }) =>
      new Promise<boolean>((resolve) => {
        let seen = false;
        let frames = 0;
        const tick = () => {
          const element = document.querySelector(selector);
          if (element !== null) {
            if (element.hasAttribute("data-starting-style")) {
              seen = true;
            } else {
              resolve(seen);
              return;
            }
          }
          frames += 1;
          if (frames > 300) {
            resolve(seen);
            return;
          }
          requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
      }),
    { selector },
  );
}

// Sample each frame until the popup is detached; resolves whether the ending
// state was observed while it was still mounted.
function observeExitStyle(page: Page) {
  return page.evaluate(
    () =>
      new Promise<boolean>((resolve) => {
        let seen = false;
        let frames = 0;
        const tick = () => {
          const popup = document.querySelector(".image-lightbox");
          if (popup === null) {
            resolve(seen);
            return;
          }
          if (popup.hasAttribute("data-ending-style")) {
            seen = true;
          }
          frames += 1;
          if (frames > 300) {
            resolve(seen);
            return;
          }
          requestAnimationFrame(tick);
        };
        requestAnimationFrame(tick);
      }),
  );
}

test("plays the enter and exit transitions around a plain image", async ({
  page,
}) => {
  await openDocument(page);
  const before = await captureInvariants(page);

  await page.locator(".m2h-image-frame").first().hover();
  const enterStage = observeEnterStyle(page, ".image-lightbox");
  const enterBackdrop = observeEnterStyle(page, ".image-lightbox-backdrop");
  await page.locator(".m2h-lightbox-trigger").first().click();

  // Both presentation layers really entered through their starting states…
  expect(await enterStage).toBe(true);
  expect(await enterBackdrop).toBe(true);
  // …and settle fully opaque while the popup stays up.
  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  await expect(popup.locator(".image-lightbox-stage")).toHaveCSS(
    "opacity",
    "1",
  );
  await expect(page.locator(".image-lightbox-backdrop")).toHaveCSS(
    "opacity",
    "1",
  );

  // Closing runs the exit transition before the popup leaves the DOM.
  const exitSeen = observeExitStyle(page);
  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  expect(await exitSeen).toBe(true);
  await expect(page.locator(".image-lightbox")).toHaveCount(0);
  await expectInvariantsUnchanged(page, before);
});

test("plays the enter and exit transitions around a mermaid diagram", async ({
  page,
}) => {
  await openMermaidDocument(page);
  const before = await captureMermaidInvariants(page);

  await page.locator(".m2h-mermaid-frame").hover();
  const enterStage = observeEnterStyle(page, ".image-lightbox");
  const enterBackdrop = observeEnterStyle(page, ".image-lightbox-backdrop");
  await page.locator(".m2h-mermaid-frame .m2h-lightbox-trigger").click();

  expect(await enterStage).toBe(true);
  expect(await enterBackdrop).toBe(true);
  await expect(page.locator(".image-lightbox-vector > svg")).toHaveCount(1);

  const exitSeen = observeExitStyle(page);
  await page.keyboard.press("Escape");
  expect(await exitSeen).toBe(true);
  await expect(page.locator(".image-lightbox")).toHaveCount(0);
  await expectInvariantsUnchanged(page, before);
});

test("animates the presentation layers but never the image transform", async ({
  page,
}) => {
  await openDocument(page);
  await openLightbox(page, 0);

  const contracts = await page.evaluate(() => {
    const read = (selector: string) => {
      const element = document.querySelector(selector);
      if (element === null) {
        throw new Error(`${selector} was not rendered`);
      }
      const style = getComputedStyle(element);
      return {
        duration: style.transitionDuration,
        property: style.transitionProperty,
      };
    };
    return {
      image: read(".image-lightbox-image"),
      stage: read(".image-lightbox-stage"),
      backdrop: read(".image-lightbox-backdrop"),
    };
  });

  // The image's live transform (pan / zoom / rotate) must never transition —
  // a transition there would lag pointer pans and blur the clamp math.
  expect(contracts.image.duration).toBe("0s");
  // The enter/exit motion rides on the stage and the backdrop instead.
  expect(contracts.stage.duration).toContain("0.18s");
  expect(contracts.stage.property).toContain("transform");
  expect(contracts.backdrop.duration).toBe("0.16s");
});

test("opens and closes without motion under prefers-reduced-motion", async ({
  page,
}) => {
  await page.emulateMedia({ reducedMotion: "reduce" });
  await openDocument(page);
  const before = await captureInvariants(page);

  await openLightbox(page, 0);
  const stage = page.locator(".image-lightbox-stage");
  await expect(stage).toHaveCSS("transition-duration", "0s");
  await expect(page.locator(".image-lightbox-backdrop")).toHaveCSS(
    "transition-duration",
    "0s",
  );

  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(page.locator(".image-lightbox")).toHaveCount(0);
  await expectInvariantsUnchanged(page, before);
});
