import { type ChildProcess, spawn } from "node:child_process";
import { once } from "node:events";
import {
  copyFileSync,
  mkdirSync,
  mkdtempSync,
  readFileSync,
  rmSync,
  writeFileSync,
} from "node:fs";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { expect, test } from "@playwright/test";

// The standard webServer builds one binary. Every case below reuses that same
// binary, exercising deployment prefixes as runtime configuration only.
for (const base of ["", "/docs", "/apps/docs"]) {
  for (const cdn of [false, true]) {
    test.describe(`${base || "/"} ${cdn ? "CDN" : "embedded"}`, () => {
      test.use({ deviceScaleFactor: 2 });
      let server: ChildProcess;
      let directory: string;
      let origin: string;
      let output = "";

      test.beforeAll(async () => {
        const socket = createServer();
        socket.listen(0, "127.0.0.1");
        await once(socket, "listening");
        const address = socket.address();
        if (address === null || typeof address === "string")
          throw new Error("missing port");
        const port = address.port;
        await new Promise<void>((resolve, reject) =>
          socket.close((error) => (error ? reject(error) : resolve())),
        );
        origin = `http://127.0.0.1:${port}`;
        directory = mkdtempSync(join(tmpdir(), "m2h-base-path-"));
        const docs = join(directory, "docs");
        const other = join(directory, "other");
        mkdirSync(docs);
        mkdirSync(other);
        const frontmatter = (title: string) =>
          `---\ntitle: ${title}\ndescription: Base path regression fixture\ntags: [test]\ncreate_date: 2026-10-28\nupdate_date: 2026-10-28\n---\n`;
        writeFileSync(
          join(docs, "README.md"),
          `${frontmatter("Base Home")}# Base Home\n\n[Guide](guide.md#section)\n\n[Outside](../secret.md)\n`,
        );
        writeFileSync(
          join(other, "README.md"),
          `${frontmatter("Other Root")}# Other Root\n`,
        );
        writeFileSync(
          join(docs, "guide.md"),
          frontmatter("Base Guide") +
            `# Base Guide

[Home](README.md)

## Section

uniquebasequery

![Logo](logo.svg)

![Missing](missing.png)

<img alt="Responsive density" src="density-1.png" srcset="data:image/png;base64,${readFileSync("e2e/docs/images/square.png").toString("base64")} 1x, /density-2.png 2x">

<img alt="Responsive width" src="density-1.png" sizes="100px" srcset="density-1.png 100w, width-2.png 200w">

<picture><source media="(min-width: 1px)" srcset="picture-2.png 2x"><img alt="Responsive picture" src="density-1.png"></picture>

$E=mc^2$

~~~mermaid
flowchart LR
  A[Start] --> B[End]
~~~

| Value |
| --- |
| 2 |
| 1 |
`,
        );
        writeFileSync(
          join(docs, "logo.svg"),
          '<svg xmlns="http://www.w3.org/2000/svg" width="120" height="60"><text x="5" y="30">Base Logo</text></svg>',
        );
        for (const name of [
          "density-1.png",
          "density-2.png",
          "width-2.png",
          "picture-2.png",
        ]) {
          copyFileSync("e2e/docs/images/square.png", join(docs, name));
        }
        writeFileSync(join(docs, ".private.md"), "# Hidden\n");
        writeFileSync(join(docs, "bad.html"), "<script>bad</script>");
        server = spawn(resolve("../build/e2e/m2h"), [
          docs,
          other,
          "--no-open",
          "--port",
          String(port),
          "--base-path",
          base || "/",
          cdn ? "--cdn" : "--no-cdn",
        ]);
        server.stdout?.on("data", (data) => {
          output += String(data);
        });
        server.stderr?.on("data", (data) => {
          output += String(data);
        });
        server.on("error", (error) => {
          output += error.message;
        });
        await expect
          .poll(async () => {
            try {
              return (await fetch(`${origin}${base}/healthz`)).status;
            } catch {
              return output || "starting";
            }
          })
          .toBe(200);
      });

      test.afterAll(async () => {
        if (server?.exitCode === null) {
          const exited = once(server, "exit");
          server.kill("SIGTERM");
          await exited;
        }
        if (directory) rmSync(directory, { recursive: true, force: true });
      });

      test("deep links, navigation, search, rich resources and boundaries preserve the prefix", async ({
        page,
        context,
        request,
      }) => {
        const localRequests: string[] = [];
        page.on("request", (request) => {
          const url = new URL(request.url());
          if (url.origin === origin) localRequests.push(url.pathname);
        });
        const errors: string[] = [];
        page.on("pageerror", (error) => errors.push(error.message));
        // Exercise the CDN choice deterministically using the exact vendored
        // release bytes; public CDN reachability is outside this regression.
        if (cdn) {
          await page.route("https://cdn.jsdelivr.net/**", async (route) => {
            const path = new URL(route.request().url()).pathname;
            const filename = path
              .split("/")
              .at(-1)
              ?.replace(/^(tablesort\.[a-z]+)\.min\.js$/, "$1.js");
            const asset = path.includes("/fonts/")
              ? `fonts/${filename}`
              : filename;
            const response = await request.get(
              `${origin}${base}/runtime/${asset}`,
            );
            expect(response.ok()).toBe(true);
            await route.fulfill({ response });
          });
        }
        await page.goto(`${origin}${base}/`);
        await expect(page).toHaveURL(`${origin}${base}/doc/r0/README.md`);
        await expect(page.locator(".markdown-body h1")).toHaveText("Base Home");
        await expect(
          page.locator('.markdown-body a:has-text("Outside")'),
        ).toHaveAttribute(
          "href",
          new RegExp(`^${base}/__m2h_invalid_local_reference__`),
        );
        await page.locator('.markdown-body a:has-text("Guide")').click();
        await expect(page).toHaveURL(
          `${origin}${base}/doc/r0/guide.md#section`,
        );
        await expect(page.locator(".markdown-body h1")).toHaveText(
          "Base Guide",
        );
        await expect(page.locator(".markdown-body .katex")).toBeVisible();
        await expect(page.locator(".mermaid svg")).toBeVisible();
        const image = page.locator(".m2h-image-frame").first();
        await image.scrollIntoViewIfNeeded();
        await expect(image.locator(".m2h-svg-image svg")).toBeVisible();
        await image.hover();
        await image.locator(".m2h-lightbox-trigger").click();
        await expect(
          page.getByRole("dialog", { name: "视觉内容预览" }),
        ).toBeVisible();
        await page.getByRole("button", { name: "放大图片" }).click();
        await page.keyboard.press("Escape");
        const missing = page.locator('img[data-m2h-fallback="true"]');
        await missing.scrollIntoViewIfNeeded();
        await expect(missing).toHaveAttribute(
          "src",
          `${base}/ui/image-load-failed.svg`,
        );
        for (const [alt, filename] of [
          ["Responsive density", "density-2.png"],
          ["Responsive width", "width-2.png"],
          ["Responsive picture", "picture-2.png"],
        ]) {
          const responsive = page.getByAltText(alt, { exact: true });
          await responsive.scrollIntoViewIfNeeded();
          await expect
            .poll(() =>
              responsive.evaluate(
                (image: HTMLImageElement) => image.currentSrc,
              ),
            )
            .toBe(`${origin}${base}/assets/r0/${filename}`);
          await expect
            .poll(() =>
              responsive.evaluate(
                (image: HTMLImageElement) => image.naturalWidth,
              ),
            )
            .toBeGreaterThan(0);
          expect(localRequests).toContain(`${base}/assets/r0/${filename}`);
        }
        await page.reload();
        await expect(page.locator(".markdown-body h1")).toHaveText(
          "Base Guide",
        );
        await page.goBack();
        await expect(page.locator(".markdown-body h1")).toHaveText("Base Home");
        await page.goForward();
        await expect(page.locator(".markdown-body h1")).toHaveText(
          "Base Guide",
        );
        await page.getByRole("button", { name: /^显示主题：/ }).click();
        await page
          .getByRole("menuitemradio", { name: "深色", exact: true })
          .click();
        await page.keyboard.press("Escape");
        expect(new URL(page.url()).pathname).toBe(`${base}/doc/r0/guide.md`);
        await page.keyboard.press("Control+k");
        await page
          .getByRole("searchbox", { name: "全文搜索" })
          .fill("uniquebasequery");
        await page
          .getByRole("dialog")
          .getByRole("button", { name: /Base Guide/ })
          .click();
        await expect(page).toHaveURL(
          new RegExp(`${base}/doc/r0/guide.md.*#section$`),
        );
        const link = page.getByRole("button", {
          name: "guide.md，r0/guide.md",
          exact: true,
        });
        await expect(link).toBeAttached();
        await context.grantPermissions(["clipboard-read", "clipboard-write"]);
        await link.click({ button: "right" });
        await page.getByRole("menuitem", { name: "复制文档网页链接" }).click();
        await expect
          .poll(() => page.evaluate(() => navigator.clipboard.readText()))
          .toBe(`${origin}${base}/doc/r0/guide.md`);
        expect(
          await (await request.get(`${origin}${base}/raw/r0/guide.md`)).text(),
        ).toContain("# Base Guide");
        expect(
          (
            await request.get(`${origin}${base}/api/document?path=r1/README.md`)
          ).ok(),
        ).toBe(true);
        for (const route of [
          "/doc/r0/.private.md",
          "/assets/r0/bad.html",
          "/api/unknown",
          "/__m2h_invalid_local_reference__",
        ]) {
          expect((await request.get(`${origin}${base}${route}`)).status()).toBe(
            404,
          );
        }
        if (base) {
          expect(
            (
              await request.get(`${origin}${base}?mode=dark`, {
                maxRedirects: 0,
              })
            ).headers().location,
          ).toBe(`${base}/?mode=dark`);
          expect((await request.get(`${origin}/api/files`)).status()).toBe(404);
          expect((await request.get(`${origin}${base}-other/`)).status()).toBe(
            404,
          );
        }
        expect(
          localRequests.some(
            (path) =>
              path.endsWith(".js") && path.startsWith(`${base}/ui/assets/`),
          ),
        ).toBe(true);
        expect(
          localRequests.some((path) => path === `${base}/ui/markdown.css`),
        ).toBe(true);
        expect(
          localRequests.filter((path) => !path.startsWith(`${base}/`)),
        ).toEqual([]);
        expect(errors).toEqual([]);
      });
    });
  }
}
