import { expect, test } from "@playwright/test";

const cases = [
  { name: "PNG", path: "image-lightbox.md", frame: ".m2h-image-frame" },
  { name: "SVG", path: "image-lightbox.md", frame: ".m2h-image-frame" },
  { name: "Mermaid", path: "mermaid-lightbox.md", frame: ".m2h-mermaid-frame" },
  {
    name: "Vega-Lite",
    path: "vega-lite-lightbox.md",
    frame: ".m2h-vega-lite-frame",
  },
];

for (const visual of cases) {
  test(`${visual.name} zooms across the full viewport`, async ({ page }) => {
    await page.setViewportSize({ width: 800, height: 600 });
    await page.goto(`/doc/${visual.path}`);
    const frame = page.locator(visual.frame).first();
    await frame.hover();
    const trigger = frame.locator(".m2h-lightbox-trigger");
    await expect(trigger).toBeVisible();
    if (visual.name === "SVG") {
      await frame.locator("img").evaluate(async (image: HTMLImageElement) => {
        image.removeAttribute("srcset");
        image.src = "/ui/image-loading.svg";
        await image.decode();
      });
    }
    await trigger.click();
    const stage = page.locator(".image-lightbox-stage");
    await expect
      .poll(() => stage.boundingBox())
      .toEqual({
        x: 0,
        y: 0,
        width: 800,
        height: 600,
      });

    if (visual.name === "PNG") {
      // Long descriptions must not resize the stage or crowd out controls.
      await page.locator(".image-lightbox-alt").evaluate((alt) => {
        alt.textContent = "Long image description ".repeat(400);
      });
      await expect(stage).toHaveCSS("height", "600px");
      await expect(
        page.getByRole("button", { name: "放大图片" }),
      ).toBeInViewport();
    }

    const zoom = page.getByRole("button", { name: "放大图片" });
    for (let step = 0; step < 8; step++) {
      await zoom.click();
    }
    await expect(zoom).toBeDisabled();
    // Hit-test outside the old inset/footer-clipped canvas. This proves the
    // zoomed visual actually paints there, beyond merely resizing its wrapper.
    const visibleAtEdge = await page.evaluate(() => {
      const visualNode = document.querySelector(
        ".image-lightbox-image, .image-lightbox-vector",
      );
      if (visualNode === null) throw new Error("missing visual");
      const rect = visualNode.getBoundingClientRect();
      const point =
        rect.height > innerHeight
          ? { x: innerWidth / 2, y: innerHeight - 2 }
          : { x: 2, y: innerHeight / 2 };
      const hit = document.elementFromPoint(point.x, point.y);
      return hit === visualNode || (hit !== null && visualNode.contains(hit));
    });
    expect(visibleAtEdge).toBe(true);

    await page.setViewportSize({ width: 390, height: 700 });
    await expect
      .poll(() => stage.boundingBox())
      .toEqual({
        x: 0,
        y: 0,
        width: 390,
        height: 700,
      });
    await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
    await expect(page.getByRole("dialog")).toBeHidden();
  });
}
