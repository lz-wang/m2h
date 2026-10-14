import { execFileSync } from "node:child_process";
import {
  copyFileSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, type Page, test } from "@playwright/test";

const panel = (id: string) => `.m2h-tabbed-panel[data-m2h-tab-id="${id}"]`;
const radio = (id: string) => `input.m2h-tabbed-radio[id="${id}"]`;

async function search(page: Page, query: string) {
  await expect(page.locator(".reader-toolbar")).toBeVisible();
  await page.keyboard.press("Control+k");
  await page.getByRole("searchbox", { name: "全文搜索" }).fill(query);
  await page
    .getByRole("dialog")
    .getByRole("button", { name: "Tabbed 回归演示，tabbed.md" })
    .click();
}

async function expectInViewport(page: Page, selector: string) {
  await expect
    .poll(() =>
      page.locator(selector).evaluate((node) => {
        const top = node.getBoundingClientRect().top;
        return top >= 0 && top < window.innerHeight;
      }),
    )
    .toBe(true);
}

async function exportFixture(page: Page, source?: string): Promise<string> {
  const dir = mkdtempSync(join(tmpdir(), "m2h-tabbed-"));
  try {
    const input = join(dir, "input.md");
    if (source === undefined) copyFileSync("e2e/docs/tabbed.md", input);
    else writeFileSync(input, source);
    execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
    const html = readFileSync(join(dir, "index.html"), "utf8");
    await page.route("**/assets/tabbed-export.html", (route) =>
      route.fulfill({ contentType: "text/html", body: html }),
    );
    await page.route("https://cdn.jsdelivr.net/**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      const filename = path.split("/").at(-1);
      const asset = path.includes("/fonts/") ? `fonts/${filename}` : filename;
      const response = await page.request.get(`/runtime/${asset}`);
      expect(response.ok()).toBe(true);
      await route.fulfill({ response });
    });
    return html;
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
}

test("independent native groups, defaults, keyboard focus and mobile width", async ({
  page,
}) => {
  await page.goto("/doc/tabbed.md");
  await expect(page.locator(".m2h-tabbed-set")).toHaveCount(3);
  await expect(page.locator(radio("m2h-tab-1-2"))).toBeChecked();
  await expect(page.locator(radio("m2h-tab-2-1"))).toBeChecked();
  await page.locator(radio("m2h-tab-1-2")).focus();
  await page.keyboard.press("ArrowLeft");
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeFocused();
  await page.locator('label[for="m2h-tab-2-2"]').click();
  await expect(page.locator(panel("m2h-tab-2-2"))).toBeVisible();
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  await page.setViewportSize({ width: 390, height: 844 });
  const overflow = await page
    .locator(".markdown-body")
    .evaluate((node) => node.scrollWidth > node.clientWidth);
  expect(overflow).toBe(false);
});

test("TOC and direct/reloaded hashes activate duplicate headings", async ({
  page,
}) => {
  await page.goto("/doc/tabbed.md#%E5%AE%89%E8%A3%85");
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  await expectInViewport(page, 'h2[id="安装"]');
  const toc = page.getByRole("navigation", { name: "文档目录" });
  await toc.locator('a[href="#%E5%AE%89%E8%A3%85-1"]').click();
  await expect(page.locator(radio("m2h-tab-1-2"))).toBeChecked();
  await expectInViewport(page, 'h2[id="安装-1"]');
  await page.reload();
  await expect(page.locator(radio("m2h-tab-1-2"))).toBeChecked();
  await expectInViewport(page, 'h2[id="安装-1"]');
});

test("cross-document Markdown links reveal hidden headings", async ({
  page,
}) => {
  await page.goto("/doc/README.md");
  await expect(page.locator(".markdown-body")).toBeVisible();
  // Browser-visible Markdown links use the same App link interception path.
  await page.locator(".markdown-body").evaluate((root) => {
    const a = document.createElement("a");
    a.href = "/doc/tabbed.md#安装";
    a.textContent = "Tabbed target";
    root.append(a);
  });
  await page.getByRole("link", { name: "Tabbed target" }).click();
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  await expectInViewport(page, 'h2[id="安装"]');
});

test("search activates headingless panels across documents and in place", async ({
  page,
}) => {
  let requests = 0;
  page.on("request", (request) => {
    if (request.url().includes("/api/document?path=tabbed.md")) requests++;
  });
  await page.goto("/doc/README.md");
  await search(page, "tabbed-headingless-token");
  await expect(page.locator(radio("m2h-tab-2-2"))).toBeChecked();
  await expectInViewport(page, panel("m2h-tab-2-2"));
  expect(requests).toBe(1);
  await page.locator('label[for="m2h-tab-2-1"]').click();
  await search(page, "tabbed-headingless-token");
  await expect(page.locator(radio("m2h-tab-2-2"))).toBeChecked();
  expect(requests).toBe(1);
  await search(page, "python-tab-unique-token");
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  await expectInViewport(page, 'h2[id="安装"]');
});

test("equal-height tab switching preserves scroll and duplicate-heading navigation", async ({
  page,
}) => {
  await page.goto("/doc/tabbed.md#%E5%AE%89%E8%A3%85");
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  const first = page.locator(panel("m2h-tab-1-1"));
  const second = page.locator(panel("m2h-tab-1-2"));
  const set = first.locator("../..");
  await set.evaluate((node) =>
    window.scrollTo(0, window.scrollY + node.getBoundingClientRect().top - 120),
  );
  await page.locator(radio("m2h-tab-1-1")).focus();
  const before = await page.evaluate(() => window.scrollY);
  const height = await first.evaluate(
    (node) => node.getBoundingClientRect().height,
  );
  for (const [key, id] of [
    ["ArrowRight", "m2h-tab-1-2"],
    ["ArrowLeft", "m2h-tab-1-1"],
    ["ArrowRight", "m2h-tab-1-2"],
  ]) {
    await page.keyboard.press(key);
    await expect(page.locator(radio(id))).toBeChecked();
    await expect(page.locator(radio(id))).toBeFocused();
    expect(
      Math.abs((await page.evaluate(() => window.scrollY)) - before),
    ).toBeLessThanOrEqual(2);
  }
  expect(
    Math.abs(
      (await second.evaluate((node) => node.getBoundingClientRect().height)) -
        height,
    ),
  ).toBeLessThanOrEqual(2);
  const toc = page.getByRole("navigation", { name: "文档目录" });
  for (const id of ["安装", "安装-1", "安装"]) {
    await toc.locator(`a[href="#${encodeURIComponent(id)}"]`).click();
    await expectInViewport(page, `h2[id="${id}"]`);
    await expect
      .poll(() => page.evaluate(() => decodeURIComponent(location.hash)))
      .toBe(`#${id}`);
    await expect(
      page.locator(radio(id === "安装" ? "m2h-tab-1-1" : "m2h-tab-1-2")),
    ).toBeChecked();
  }
});

test("different-height tab switching keeps the group visible and retains focus and navigation", async ({
  page,
}) => {
  await page.goto("/doc/tabbed.md");
  const set = page.locator(panel("m2h-tab-2-1")).locator("../..");
  await expect(set).toBeVisible();
  await set.evaluate((node) =>
    window.scrollTo(0, window.scrollY + node.getBoundingClientRect().top - 120),
  );
  await page.locator(radio("m2h-tab-2-1")).focus();
  const shortHeight = await set.evaluate(
    (node) => node.getBoundingClientRect().height,
  );
  await page.keyboard.press("ArrowRight");
  await expect(page.locator(radio("m2h-tab-2-2"))).toBeChecked();
  await expect(
    page.locator(panel("m2h-tab-2-2")).locator(".mermaid svg"),
  ).toBeVisible();
  expect(
    await set.evaluate((node) => node.getBoundingClientRect().height),
  ).toBeGreaterThan(shortHeight + 100);
  for (const [key, id] of [
    ["ArrowLeft", "m2h-tab-2-1"],
    ["ArrowRight", "m2h-tab-2-2"],
  ]) {
    await page.keyboard.press(key);
    await expect(page.locator(radio(id))).toBeChecked();
    await expect(page.locator(radio(id))).toBeFocused();
    await expect
      .poll(() =>
        set.evaluate((node) => {
          const box = node.getBoundingClientRect();
          return box.top >= 0 && box.top < window.innerHeight;
        }),
      )
      .toBe(true);
    expect(await page.evaluate(() => location.pathname)).toBe("/doc/tabbed.md");
  }
});

test("hidden rich content, sorting, lazy image and Lightbox work on first activation", async ({
  page,
}) => {
  await page.goto("/doc/tabbed.md");
  const hidden = page.locator(panel("m2h-tab-2-2"));
  await expect(hidden.locator(".mermaid svg")).toHaveCount(1);
  await page.locator('label[for="m2h-tab-2-2"]').click();
  await expect(hidden.locator(".mermaid svg")).toBeVisible();
  expect(
    await hidden
      .locator(".mermaid svg")
      .evaluate((node) => node.getBoundingClientRect().width),
  ).toBeGreaterThan(20);
  await expect(hidden.locator(".katex")).toBeVisible();
  await expect(hidden.locator("img")).toBeVisible();
  await expect
    .poll(() =>
      hidden
        .locator("img")
        .evaluate((img: HTMLImageElement) => img.naturalWidth),
    )
    .toBeGreaterThan(0);
  await hidden.getByRole("columnheader", { name: /名称/ }).click();
  await expect(hidden.locator("tbody tr").first()).toContainText("alpha");
  await hidden.locator(".m2h-rich-visual-frame").hover();
  await hidden.getByRole("button", { name: "查看 Mermaid 图表" }).click();
  await expect(
    page.getByRole("dialog", { name: "视觉内容预览" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
});

test("theme changes preserve selection, native focus and scroll", async ({
  page,
}) => {
  await page.goto("/doc/tabbed.md#%E5%AE%89%E8%A3%85");
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  const before = await page.evaluate(() => window.scrollY);
  for (const theme of ["深色", "浅色", "跟随系统"]) {
    await page.getByRole("button", { name: /^显示主题：/ }).click();
    await page.getByRole("menuitemradio", { name: theme, exact: true }).click();
    await page.keyboard.press("Escape");
    await expect(
      page.getByRole("menuitemradio", { name: theme, exact: true }),
    ).toBeHidden();
    await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  }
  expect(
    Math.abs((await page.evaluate(() => window.scrollY)) - before),
  ).toBeLessThan(100);
  await page.locator(radio("m2h-tab-1-1")).focus();
  await page.emulateMedia({ colorScheme: "dark" });
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeFocused();
});

test("export restores hashes, renders hidden diagrams and prints every panel", async ({
  page,
}) => {
  await exportFixture(page);
  await page.goto("/assets/tabbed-export.html#%E5%AE%89%E8%A3%85");
  await expect(page.locator(radio("m2h-tab-1-1"))).toBeChecked();
  await expect(page.locator(".mermaid svg")).toHaveCount(1);
  await page.evaluate(() => {
    location.hash = "安装-1";
  });
  await expect(page.locator(radio("m2h-tab-1-2"))).toBeChecked();
  await page.locator('label[for="m2h-tab-2-2"]').click();
  await expect(page.locator(".mermaid svg")).toBeVisible();
  await page.emulateMedia({ media: "print" });
  for (const item of await page.locator(".m2h-tabbed-panel").all())
    await expect(item).toBeVisible();
  await expect(page.locator(".m2h-tabbed-labels").first()).toBeHidden();
});

test("export radios switch without JavaScript and 33 tabs keep all content", async ({
  browser,
}) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  try {
    const html = await exportFixture(
      page,
      '=== "One"\n\n    first\n\n=== "Two"\n\n    second\n',
    );
    expect(html).not.toContain("cdn.jsdelivr.net");
    await page.goto("http://127.0.0.1:8873/assets/tabbed-export.html");
    await expect(page.locator(panel("m2h-tab-1-1"))).toBeVisible();
    await page.locator('label[for="m2h-tab-1-2"]').click();
    await expect(page.locator(panel("m2h-tab-1-2"))).toBeVisible();
    await expect(page.locator(panel("m2h-tab-1-1"))).toBeHidden();
    let atLimit = "";
    for (let i = 0; i < 32; i++)
      atLimit += `=== "Tab ${i}"\n\n    content-${i}\n\n`;
    await exportFixture(page, atLimit);
    await page.reload();
    await page.locator('label[for="m2h-tab-1-32"]').click();
    await expect(page.locator(panel("m2h-tab-1-32"))).toBeVisible();
    await expect(page.locator(panel("m2h-tab-1-1"))).toBeHidden();
    let source = "";
    for (let i = 0; i < 33; i++)
      source += `=== "Tab ${i}"\n\n    content-${i}\n\n`;
    await exportFixture(page, source);
    await page.reload();
    await expect(page.locator(".m2h-tabbed-radio")).toHaveCount(0);
    await expect(page.locator(".m2h-tabbed-panel")).toHaveCount(33);
    await expect(page.getByText("content-32", { exact: true })).toBeVisible();
  } finally {
    await context.close();
  }
});
