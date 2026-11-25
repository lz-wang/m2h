import { execFileSync } from "node:child_process";
import { copyFileSync, mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { pathToFileURL } from "node:url";
import { expect, type Locator, type Page, test } from "@playwright/test";

function textLineCounts(cells: Locator) {
  return cells.evaluateAll((nodes) =>
    nodes.map((node) => {
      const range = document.createRange();
      range.selectNodeContents(
        node.querySelector("code") ?? node.firstChild ?? node,
      );
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
  const header = headers.first();
  const firstCell = rows.first().locator("td").first();
  const sortButton = header.getByRole("button", { name: /字段：升序排序/ });
  await expect(sortButton).toBeVisible();
  const text = await header.evaluate((cell) => {
    const range = document.createRange();
    range.selectNode(cell.firstChild ?? cell);
    const { x, y, width, height } = range.getBoundingClientRect();
    return { x, y, width, height };
  });
  const buttonBox = await sortButton.boundingBox();
  expect(buttonBox?.x).toBeGreaterThan(text.x + text.width);
  await page.mouse.click(text.x + text.width / 2, text.y + text.height / 2);
  await header.click({ position: { x: 2, y: 2 } });
  await expect(header).toHaveAttribute("aria-sort", "none");
  await expect(firstCell).toHaveText("beta");
  const bodyTextStyle = await firstCell.evaluate((cell) => {
    const style = getComputedStyle(cell);
    return { cursor: style.cursor, userSelect: style.userSelect };
  });
  expect(
    await header.evaluate((cell) => {
      const style = getComputedStyle(cell);
      return { cursor: style.cursor, userSelect: style.userSelect };
    }),
  ).toEqual(bodyTextStyle);
  await page.mouse.move(text.x + 1, text.y + text.height / 2);
  await page.mouse.down();
  await page.mouse.move(text.x + text.width, text.y + text.height / 2, {
    steps: 5,
  });
  await page.mouse.up();
  expect(
    await page.evaluate(() => window.getSelection()?.toString()),
  ).toContain("字段");
  await expect(header).toHaveAttribute("aria-sort", "none");

  await sortButton.click();
  await expect(header).toHaveAttribute("aria-sort", "ascending");
  await expect(firstCell).toHaveText("alpha");
  const focusedButton = header.getByRole("button", { name: /字段：当前升序/ });
  await focusedButton.focus();
  await page.keyboard.press("Space");
  await expect(header).toHaveAttribute("aria-sort", "descending");
  await expect(firstCell).toHaveText("beta");
  await page.keyboard.press("Enter");
  await expect(header).toHaveAttribute("aria-sort", "ascending");
  await expect(firstCell).toHaveText("alpha");
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

test("keeps table headers on one line and sorts from the icon in the reader", async ({
  page,
}) => {
  await page.goto("/doc/table-layout.md");
  await expectTableLayout(page);
});

test("keeps table headers on one line and sorts from the icon in exported HTML", async ({
  page,
}) => {
  const outputDir = mkdtempSync(join(tmpdir(), "m2h-table-layout-"));
  try {
    const input = join(outputDir, "input.md");
    copyFileSync("e2e/docs/table-layout.md", input);
    execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
    await page.route("https://cdn.jsdelivr.net/**", async (route) => {
      const filename = new URL(route.request().url()).pathname
        .split("/")
        .at(-1);
      await route.fulfill({
        response: await page.request.get(`/runtime/${filename}`),
      });
    });
    await page.goto(pathToFileURL(join(outputDir, "index.html")).href);
    await expectTableLayout(page);
  } finally {
    rmSync(outputDir, { recursive: true, force: true });
  }
});
