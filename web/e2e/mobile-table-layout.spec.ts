import { expect, test } from "@playwright/test";

test("keeps sortable table headers readable and tappable on touch screens", async ({
  page,
}) => {
  await page.goto("/doc/table-layout.md");
  const table = page.locator(".markdown-body table:not([class])");
  const headers = table.locator("thead th");
  const button = headers.first().getByRole("button");
  const keys = table.locator("tbody td:first-child");
  await expect(button).toBeVisible();
  await expect(button).toHaveAttribute("data-m2h-decor", "table-sort");
  expect(
    await page.evaluate(() => matchMedia("(pointer: coarse)").matches),
  ).toBe(true);

  const rangeLines = await headers.evaluateAll((cells) =>
    cells.map((cell) => {
      const range = document.createRange();
      range.selectNodeContents(
        cell.querySelector("code") ?? cell.firstChild ?? cell,
      );
      return new Set(Array.from(range.getClientRects(), ({ top }) => top)).size;
    }),
  );
  expect(rangeLines).toEqual([1, 1, 1, 1, 1]);
  const widths = () =>
    headers.evaluateAll((cells) =>
      cells.map((cell) => cell.getBoundingClientRect().width),
    );
  const before = await widths();
  const target = await button.evaluate((element) => {
    const cell = element.closest("th")?.getBoundingClientRect();
    if (!cell) throw new Error("missing table header");
    const { top, bottom, width, height } = element.getBoundingClientRect();
    return {
      width,
      height,
      staysInCell: top >= cell.top && bottom <= cell.bottom,
    };
  });
  expect(target).toEqual({ width: 32, height: 32, staysInCell: true });

  const mask = () =>
    button.evaluate(
      (element) => getComputedStyle(element, "::after").maskImage,
    );
  const unsorted = await mask();
  await expect(headers.first()).toHaveAttribute("aria-sort", "none");
  await button.tap();
  await expect(keys).toHaveText(["alpha", "beta"]);
  await expect(headers.first()).toHaveAttribute("aria-sort", "ascending");
  const ascending = await mask();
  await button.tap();
  await expect(keys).toHaveText(["beta", "alpha"]);
  await expect(headers.first()).toHaveAttribute("aria-sort", "descending");
  const descending = await mask();
  expect(
    [unsorted, ascending, descending].every((icon) => icon.startsWith("url(")),
  ).toBe(true);
  expect(new Set([unsorted, ascending, descending]).size).toBe(3);
  expect(await widths()).toEqual(before);

  expect(
    await table.evaluate((element) => {
      element.scrollLeft = element.scrollWidth;
      return element.scrollLeft;
    }),
  ).toBeGreaterThan(0);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    page.viewportSize()?.width,
  );
});
