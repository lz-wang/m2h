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
    const requestedAssets: string[] = [];
    await page.route("https://cdn.jsdelivr.net/**", async (route) => {
      expect(route.request().url()).not.toMatch(removedRuntime);
      const path = new URL(route.request().url()).pathname;
      const filename = path.split("/").at(-1);
      if (filename !== undefined) requestedAssets.push(filename);
      const asset = path.includes("/fonts/") ? `fonts/${filename}` : filename;
      const response = await page.request.get(`/runtime/${asset}`);
      expect(response.ok()).toBe(true);
      await route.fulfill({ response });
    });
    await page.goto("/assets/removed-export.html");
    await expect(page.locator(".mermaid > svg")).toHaveCount(1);
    await expect(page.locator(removedCode)).toHaveCount(5);
    expect(await page.locator(removedCode).allTextContents()).toEqual([
      '{"$schema":"https://vega.github.io/schema/vega-lite/v6.json","mark":"bar","description":"removed-renderer-search-token"}\n',
      "{ invalid JSON\n",
      "\nzenuml\n  Alice->Bob: Hello\n",
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
    expect(requestedAssets).toEqual(
      expect.arrayContaining([
        "katex.min.css",
        "katex.min.js",
        "auto-render.min.js",
        "mermaid.min.js",
        "tablesort.min.js",
      ]),
    );
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
});

test("code-only documents do not load Mermaid or KaTeX", async ({ page }) => {
  const runtimeRequests: string[] = [];
  page.on("request", (request) => {
    if (new URL(request.url()).pathname.startsWith("/runtime/")) {
      runtimeRequests.push(request.url());
    }
  });
  await page.goto("/doc/runtime-code-only.md");
  await expect(page.locator(".m2h-code-copy")).toHaveCount(2);
  await expect(page.locator("pre > code.language-mermaid")).toHaveText(
    "zenuml\n  Alice->Bob: Hello\n",
  );
  await expect(
    page.locator('script[src*="/runtime/"], link[href*="/runtime/"]'),
  ).toHaveCount(0);
  await expect(
    page.locator(".mermaid, .katex, .m2h-rich-visual-error"),
  ).toHaveCount(0);
  expect(runtimeRequests).toEqual([]);
});

for (const mixed of [false, true]) {
  test(`export ${mixed ? "mixed Mermaid" : "pure ZenUML"} loads only needed runtimes`, async ({
    page,
  }) => {
    const dir = mkdtempSync(join(tmpdir(), "m2h-runtime-"));
    try {
      const input = join(dir, "input.md");
      writeFileSync(
        input,
        "# Export runtime\n\n```mermaid\nzenuml\n  Alice->Bob: Hello\n```\n" +
          (mixed
            ? "\n```mermaid\nflowchart LR\n  A[zenuml label] --> B\n```\n"
            : ""),
      );
      execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
      const html = readFileSync(join(dir, "index.html"), "utf8");
      expect(html.includes("/mermaid@11.16.1/")).toBe(mixed);
      await page.route("**/assets/runtime-export.html", (route) =>
        route.fulfill({ contentType: "text/html", body: html }),
      );
      const runtimeRequests: string[] = [];
      await page.route("https://cdn.jsdelivr.net/**", async (route) => {
        runtimeRequests.push(route.request().url());
        const response = await page.request.get("/runtime/mermaid.min.js");
        await route.fulfill({ response });
      });
      await page.goto("/assets/runtime-export.html");
      await expect(page.locator("pre > code.language-mermaid")).toHaveCount(1);
      await expect(page.locator("pre > code.language-mermaid")).toHaveText(
        "zenuml\n  Alice->Bob: Hello\n",
      );
      await expect(page.locator(".mermaid > svg")).toHaveCount(mixed ? 1 : 0);
      expect(runtimeRequests).toHaveLength(mixed ? 1 : 0);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
}

for (const item of [
  {
    name: "schema fence",
    source:
      '```vega-lite\n{"$schema":"https://example.com/schema.json","mark":"bar"}\n```\n',
    code: '{"$schema":"https://example.com/schema.json","mark":"bar"}\n',
  },
  { name: "inline math code", source: "`$E=mc^2$`\n", code: "$E=mc^2$" },
]) {
  test(`export ${item.name} keeps source without KaTeX requests`, async ({
    page,
  }) => {
    const dir = mkdtempSync(join(tmpdir(), "m2h-code-export-"));
    try {
      const input = join(dir, "input.md");
      writeFileSync(input, "# Code only\n\n" + item.source);
      execFileSync("../build/e2e/m2h", ["export", input, "-o", "index.html"]);
      const html = readFileSync(join(dir, "index.html"), "utf8");
      expect(html).not.toContain("https://cdn.jsdelivr.net/");
      await page.route("**/assets/code-export.html", (route) =>
        route.fulfill({ contentType: "text/html", body: html }),
      );
      const runtimeRequests: string[] = [];
      page.on("request", (request) => {
        if (request.url().startsWith("https://cdn.jsdelivr.net/"))
          runtimeRequests.push(request.url());
      });
      await page.goto("/assets/code-export.html");
      await expect(page.locator(".markdown-body code")).toHaveCount(1);
      expect(await page.locator(".markdown-body code").textContent()).toBe(
        item.code,
      );
      await expect(
        page.locator('script[src], link[rel="stylesheet"]'),
      ).toHaveCount(0);
      await expect(page.locator(".katex")).toHaveCount(0);
      expect(runtimeRequests).toEqual([]);
    } finally {
      rmSync(dir, { recursive: true, force: true });
    }
  });
}
