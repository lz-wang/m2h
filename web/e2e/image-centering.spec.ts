import { execFileSync } from "node:child_process";
import { copyFileSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, type Page, test } from "@playwright/test";

async function expectCenteredImages(page: Page) {
  const images = page.locator(".markdown-body img");
  await expect(images).toHaveCount(7);
  for (const image of await images.all()) {
    await image.scrollIntoViewIfNeeded();
    await expect
      .poll(() =>
        image.evaluate(
          (node: HTMLImageElement) =>
            node.complete &&
            node.naturalWidth > 0 &&
            node.dataset.m2hLazyState !== "pending" &&
            node.closest(".m2h-image-failed") === null,
        ),
      )
      .toBe(true);
  }
  await expect(page.locator(".mermaid > svg")).toHaveCount(1);
  await expect(page.locator(".m2h-vega-lite > svg")).toHaveCount(1);

  const geometry = await page.evaluate(() => {
    const body = document.querySelector(".markdown-body");
    if (body === null) throw new Error("missing document");
    const bodyRect = body.getBoundingClientRect();
    const center = (bodyRect.left + bodyRect.right) / 2;
    return Array.from(
      body.querySelectorAll(
        'img, svg[role="img"]:not(svg svg), .mermaid > svg, .m2h-vega-lite > svg',
      ),
    ).map((visual) => {
      const rect = visual.getBoundingClientRect();
      return {
        label: visual.getAttribute("alt") ?? visual.getAttribute("aria-label"),
        delta: Math.abs((rect.left + rect.right) / 2 - center),
        width: rect.width,
      };
    });
  });
  expect(geometry).toHaveLength(11);
  for (const visual of geometry) {
    expect(visual.width, visual.label ?? "SVG").toBeGreaterThan(0);
    expect(visual.delta, visual.label ?? "SVG").toBeLessThanOrEqual(1);
  }
}

test("centers every document image and chart at desktop and narrow widths", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto("/doc/image-centering.md");
  await expectCenteredImages(page);
  await page.setViewportSize({ width: 390, height: 844 });
  await expectCenteredImages(page);
});

test("centers the same images and charts in exported HTML", async ({
  page,
}) => {
  const outputDir = mkdtempSync(join(tmpdir(), "m2h-centering-"));
  try {
    const output = join(outputDir, "index.html");
    const input = join(outputDir, "input.md");
    copyFileSync("e2e/docs/image-centering.md", input);
    execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
    const html = readFileSync(output, "utf8");
    await page.route("**/assets/image-centering-export.html", (route) =>
      route.fulfill({ contentType: "text/html", body: html }),
    );
    // Run the exported bootstrap with the same vendored runtime versions,
    // without requiring CDN network access during layout regression tests.
    await page.route("https://cdn.jsdelivr.net/**", async (route) => {
      const filename = new URL(route.request().url()).pathname
        .split("/")
        .at(-1);
      const response = await page.request.get(`/runtime/${filename}`);
      await route.fulfill({ response });
    });
    await page.setViewportSize({ width: 1280, height: 900 });
    await page.goto("/assets/image-centering-export.html");
    await expectCenteredImages(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await expectCenteredImages(page);
  } finally {
    rmSync(outputDir, { recursive: true, force: true });
  }
});
