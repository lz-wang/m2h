import { execFileSync } from "node:child_process";
import { copyFileSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, type Locator, type Page, test } from "@playwright/test";

function textLineCounts(cells: Locator) {
  return cells.evaluateAll((nodes) =>
    nodes.map((node) => {
      const range = document.createRange();
      range.selectNodeContents(node.querySelector("code") ?? node);
      return new Set(Array.from(range.getClientRects(), ({ top }) => top)).size;
    }),
  );
}

async function expectTableLayout(page: Page) {
  const table = page.locator(".markdown-body table:not([class])");
  const headers = table.locator("thead th");
  const rows = table.locator("tbody tr");
  await expect(table).toHaveAttribute("data-m2h-sortable", "true");
  await page.evaluate(() => document.fonts.ready.then(() => undefined));

  const columnWidths = () =>
    headers.evaluateAll((cells) =>
      cells.map((cell) => cell.getBoundingClientRect().width),
    );

  expect(await textLineCounts(headers)).toEqual([1, 1, 1, 1, 1]);
  expect(
    await table.evaluate((node) => node.scrollWidth - node.clientWidth),
  ).toBe(0);
  const widthsBeforeSort = await columnWidths();
  await headers.first().click();
  await expect(headers.first()).toHaveAttribute("aria-sort", "ascending");
  await expect(rows.first().locator("td").first()).toHaveText("alpha");
  expect(await columnWidths()).toEqual(widthsBeforeSort);

  await page.setViewportSize({ width: 390, height: 844 });
  expect(await textLineCounts(headers)).toEqual([1, 1, 1, 1, 1]);
  const [descriptionLines] = await textLineCounts(
    rows.last().locator("td").last(),
  );
  expect(descriptionLines).toBeGreaterThan(1);
  expect(
    await table.evaluate((node) => {
      node.scrollLeft = node.scrollWidth;
      return node.scrollLeft;
    }),
  ).toBeGreaterThan(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    390,
  );

  const [customHeaderLines] = await textLineCounts(
    page.locator(".markdown-body table.custom-layout th"),
  );
  expect(customHeaderLines).toBeGreaterThan(1);
}

test.use({ viewport: { width: 1440, height: 900 } });

test("keeps table headers on one line in the document reader", async ({
  page,
}) => {
  await page.goto("/doc/table-layout.md");
  await expectTableLayout(page);
});

test("keeps table headers on one line in exported HTML", async ({ page }) => {
  const outputDir = mkdtempSync(join(tmpdir(), "m2h-table-layout-"));
  try {
    const input = join(outputDir, "input.md");
    copyFileSync("e2e/docs/table-layout.md", input);
    execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
    const html = readFileSync(join(outputDir, "index.html"), "utf8");
    await page.route("**/assets/table-layout-export.html", (route) =>
      route.fulfill({ contentType: "text/html", body: html }),
    );
    await page.route("https://cdn.jsdelivr.net/**", async (route) => {
      const filename = new URL(route.request().url()).pathname
        .split("/")
        .at(-1);
      await route.fulfill({
        response: await page.request.get(`/runtime/${filename}`),
      });
    });
    await page.goto("/assets/table-layout-export.html");
    await expectTableLayout(page);
  } finally {
    rmSync(outputDir, { recursive: true, force: true });
  }
});
