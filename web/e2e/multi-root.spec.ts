import { type ChildProcess, spawn } from "node:child_process";
import path from "node:path";
import { expect, test } from "@playwright/test";

// Real-browser coverage for a multi-root workspace. The config's shared
// preview server serves the single-root fixture tree; this spec starts a
// second `m2h web root-a,root-b` process on its own port once that build
// exists and tears it down afterwards — same binary, comma-separated roots,
// exactly the CLI shape multi-root users type.

const port = 8874;
const baseURL = `http://127.0.0.1:${port}`;

let server: ChildProcess | null = null;

test.beforeAll(async () => {
  server = spawn(
    "./build/e2e/m2h",
    [
      "web",
      "--no-open",
      "--host",
      "127.0.0.1",
      "--port",
      String(port),
      "web/e2e/root-a,web/e2e/root-b",
    ],
    {
      cwd: path.resolve(import.meta.dirname, "..", ".."),
      stdio: "ignore",
    },
  );
  const deadline = Date.now() + 15_000;
  for (;;) {
    try {
      const response = await fetch(`${baseURL}/api/files`);
      if (response.ok) {
        break;
      }
    } catch {
      // Not listening yet — retry until the deadline.
    }
    if (Date.now() > deadline) {
      throw new Error("multi-root preview server did not become ready");
    }
    await new Promise((resolve) => {
      setTimeout(resolve, 200);
    });
  }
});

test.afterAll(() => {
  server?.kill("SIGTERM");
});

async function openWorkspace(
  page: import("@playwright/test").Page,
  target = "/",
) {
  await page.goto(baseURL + target);
  await page.waitForFunction(
    () => document.querySelector(".reader-document h1") !== null,
  );
}

test("lists both roots side by side and opens each same-named README", async ({
  page,
}) => {
  await openWorkspace(page);

  // Both roots render as labeled top-level rows, expanded by default; the
  // workspace default opens the primary root's document.
  await expect(page.getByRole("button", { name: "root-a" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  await expect(page.getByRole("button", { name: "root-b" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  await expect(page.locator(".markdown-body h1")).toHaveText("Root A Readme");
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r0/README.md",
  );

  // The second root's same-named README opens under its own virtual path.
  await page
    .getByRole("button", { name: "Root B Readme，r1/README.md" })
    .click();
  await expect(page.locator(".markdown-body h1")).toHaveText("Root B Readme");
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r1/README.md",
  );

  // And back to the first root's copy.
  await page
    .getByRole("button", { name: "Root A Readme，r0/README.md" })
    .click();
  await expect(page.locator(".markdown-body h1")).toHaveText("Root A Readme");
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r0/README.md",
  );
});

test("serves each root its own image; unprefixed and cross-root assets 404", async ({
  page,
}) => {
  // The fixture logos differ by size (480px vs 120px wide), so the rendered
  // natural width proves which root's file the browser actually loaded.
  await openWorkspace(page, "/doc/r0/README.md");
  const logoA = page.locator(".markdown-body img");
  await expect(logoA).toHaveAttribute("src", "/assets/r0/images/logo.png");
  await expect
    .poll(() => logoA.evaluate((image) => image.naturalWidth))
    .toBe(480);

  await openWorkspace(page, "/doc/r1/README.md");
  const logoB = page.locator(".markdown-body img");
  await expect(logoB).toHaveAttribute("src", "/assets/r1/images/logo.png");
  await expect
    .poll(() => logoB.evaluate((image) => image.naturalWidth))
    .toBe(120);

  // A multi-root workspace only serves prefixed asset routes.
  const unprefixed = await page.request.get(
    `${baseURL}/assets/images/logo.png`,
  );
  expect(unprefixed.status()).toBe(404);
  const unknownRoot = await page.request.get(
    `${baseURL}/assets/r9/images/logo.png`,
  );
  expect(unknownRoot.status()).toBe(404);
});

test("internal Markdown links stay inside their own root", async ({ page }) => {
  await openWorkspace(page, "/doc/r1/README.md");

  await page.getByRole("link", { name: "阅读 B 指南" }).click();
  await expect(page.locator(".markdown-body h1")).toHaveText("B Guide");
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r1/guide.md",
  );

  // The first root's same-named guide stays a different document.
  await openWorkspace(page, "/doc/r0/README.md");
  await page.getByRole("link", { name: "阅读 A 指南" }).click();
  await expect(page.locator(".markdown-body h1")).toHaveText("A Guide");
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r0/guide.md",
  );
});

test("a deep link with a fragment survives a reload into the same root", async ({
  page,
}) => {
  await openWorkspace(
    page,
    `/doc/r1/README.md#${encodeURIComponent("目标章节")}`,
  );
  await expect(page.locator(".markdown-body h1")).toHaveText("Root B Readme");
  await expect
    .poll(() => page.evaluate(() => window.location.pathname))
    .toBe("/doc/r1/README.md");

  await page.reload();
  await page.waitForFunction(
    () => document.querySelector(".reader-document h1") !== null,
  );
  await expect(page.locator(".markdown-body h1")).toHaveText("Root B Readme");
  await expect
    .poll(() => page.evaluate(() => window.location.pathname))
    .toBe("/doc/r1/README.md");
  await expect
    .poll(() => page.evaluate(() => window.location.hash))
    .toBe(encodeURI("#目标章节"));
});

test("search matches a root's name and keeps results grouped by root", async ({
  page,
}) => {
  await openWorkspace(page);

  // Both roots hold two documents each.
  await expect(page.getByText("4 个 Markdown 文件")).toBeVisible();

  // Matching the second root's name surfaces every document under it and
  // nothing from the first root.
  const search = page.getByRole("searchbox", { name: "搜索文档" });
  await search.fill("root-b");
  await expect(
    page.getByRole("button", { name: "Root B Readme，r1/README.md" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "B Guide，r1/guide.md" }),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: "Root A Readme，r0/README.md" }),
  ).toBeHidden();
  await expect(page.getByText("2 个 Markdown 文件")).toBeVisible();
});
