import { execFileSync } from "node:child_process";
import { copyFileSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { expect, test } from "@playwright/test";

const removedCode =
  "pre > code:is(.language-vega-lite, .language-vegalite, .language-zenuml, .language-mermaid)";
const removedRuntime = /vega|zenuml/i;

test("keeps removed formats as enhanced code across themes without runtime requests", async ({
  page,
}) => {
  const requests: string[] = [];
  page.on("request", (request) => {
    if (
      removedRuntime.test(request.url()) &&
      /runtime|cdn.jsdelivr/.test(request.url())
    )
      requests.push(request.url());
  });
  await page.context().grantPermissions(["clipboard-read", "clipboard-write"]);
  await page.goto("/doc/removed-renderers.md");
  await expect(page.locator(".mermaid > svg")).toHaveCount(1);
  await expect(page.locator(".katex").first()).toBeVisible();
  const codes = page.locator(removedCode);
  await expect(codes).toHaveCount(5);
  const source = await codes.allTextContents();
  await expect(page.locator(".m2h-code-copy")).toHaveCount(5);
  await expect(page.locator(".m2h-code-line-numbers")).toHaveCount(5);
  await expect(page.locator(".m2h-code-toggle")).toHaveCount(1);
  await page
    .getByRole("button", { name: "复制代码", exact: true })
    .first()
    .click();
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
    source[0],
  );
  await page.getByRole("button", { name: "展开代码" }).click();
  await expect(page.getByRole("button", { name: "折叠代码" })).toBeVisible();
  await page.getByRole("button", { name: /^显示主题：/ }).click();
  await page.getByRole("menuitemradio", { name: "深色", exact: true }).click();
  await expect(page.locator("html")).toHaveClass(/dark/);
  await expect(page.locator(".mermaid > svg")).toHaveCount(1);
  expect(await codes.allTextContents()).toEqual(source);
  await expect(page.locator(".m2h-rich-visual-error")).toHaveCount(0);
  await expect(page.locator("[data-m2h-lightbox-item='true']")).toHaveCount(1);
  expect(requests).toEqual([]);
  const search = await page.request.get(
    "/api/search?q=removed-renderer-search-token",
  );
  expect(search.ok()).toBe(true);
  expect(await search.text()).toContain("removed-renderers.md");
});

test("export preserves removed code and renders retained rich content", async ({
  page,
}) => {
  const dir = mkdtempSync(join(tmpdir(), "m2h-removed-"));
  try {
    const input = join(dir, "input.md");
    copyFileSync("e2e/docs/removed-renderers.md", input);
    execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
    const html = readFileSync(join(dir, "index.html"), "utf8");
    expect(html).not.toMatch(
      /vega(?:-lite|-embed)?@|mermaid-zenuml@|vegaEmbed|registerExternalDiagrams/,
    );
    await page.route("**/assets/removed-export.html", (route) =>
      route.fulfill({ contentType: "text/html", body: html }),
    );
    await page.route("https://cdn.jsdelivr.net/**", async (route) => {
      expect(route.request().url()).not.toMatch(removedRuntime);
      const path = new URL(route.request().url()).pathname;
      const filename = path.split("/").at(-1);
      const asset = path.includes("/fonts/") ? `fonts/${filename}` : filename;
      const response = await page.request.get(`/runtime/${asset}`);
      expect(response.ok()).toBe(true);
      await route.fulfill({ response });
    });
    await page.goto("/assets/removed-export.html");
    await expect(page.locator(".mermaid > svg")).toHaveCount(1);
    await expect(page.locator(removedCode)).toHaveCount(5);
    expect(await page.locator(removedCode).allTextContents()).toEqual([
      '{"mark":"bar","description":"removed-renderer-search-token"}\n',
      "{ invalid JSON\n",
      " \nzenuml\n  Alice->Bob: Hello\n",
      "Alice->Bob: Hello\n",
      Array.from(
        { length: 30 },
        (_, index) => `line ${String(index + 1).padStart(2, "0")}\n`,
      ).join(""),
    ]);
    await expect(page.locator(".katex").first()).toBeVisible();
    await expect(page.locator("table[data-m2h-sortable='true']")).toHaveCount(
      1,
    );
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});
