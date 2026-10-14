import { expect, type Locator, type Page, test } from "@playwright/test";

async function selectText(page: Page, text: Locator) {
  const box = await text.boundingBox();
  if (box === null) throw new Error("missing SVG text geometry");
  await page.evaluate(() => window.getSelection()?.removeAllRanges());
  await page.mouse.move(box.x + 1, box.y + box.height / 2);
  await page.mouse.down();
  await page.mouse.move(box.x + box.width - 1, box.y + box.height / 2, {
    steps: 20,
  });
  await page.mouse.up();
  await expect
    .poll(() => page.evaluate(() => window.getSelection()?.toString() ?? ""))
    .toContain("Internet");
}

test("ordinary and picture SVGs retain native text in the article and zoomed Lightbox", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/doc/svg-images.md");
  const frames = page.locator(".m2h-image-frame");
  await expect(frames).toHaveCount(2);
  for (const frame of await frames.all()) {
    await frame.scrollIntoViewIfNeeded();
    const svg = frame.locator(".m2h-svg-image svg");
    await expect(svg).toBeVisible();
    await selectText(page, svg.locator("text").first());
    await frame.hover();
    await frame.locator(".m2h-lightbox-trigger").click();
    const vector = page.locator(".image-lightbox-vector");
    const original = await vector.boundingBox();
    if (original === null) throw new Error("missing SVG viewport");
    await page.getByRole("button", { name: "放大图片" }).click();
    await expect
      .poll(async () => (await vector.boundingBox())?.width ?? 0)
      .toBeCloseTo(original.width * 1.25, 0);
    await expect(page.locator(".image-lightbox-image")).toHaveCount(0);
    await expect(page.locator(".image-lightbox-info")).toHaveCount(0);
    await selectText(page, vector.locator("svg text").first());
    const transform = await page
      .locator(".image-lightbox-vector-transform")
      .evaluate((node: HTMLElement) => node.style.transform);
    expect(transform).not.toContain("scale(");
    await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
    await expect(page.getByRole("dialog")).toBeHidden();
  }
});
