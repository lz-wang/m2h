import { type ChildProcess, spawn } from "node:child_process";
import path from "node:path";
import { expect, test } from "@playwright/test";

// Copy actions assert the real clipboard contents: 127.0.0.1 is a secure
// context, so copyText() takes the async Clipboard API path and readback needs
// the granted permissions.
test.use({ permissions: ["clipboard-read", "clipboard-write"] });

// Real-browser coverage for a multi-root workspace. The config's shared
// document server serves the single-root fixture tree; this spec starts a
// second `m2h root-a,root-b` process on its own port once that build
// exists and tears it down afterwards — same binary, comma-separated roots,
// exactly the CLI shape multi-root users type.

const port = 8874;
const baseURL = `http://127.0.0.1:${port}`;

let server: ChildProcess | null = null;

test.beforeAll(async () => {
  server = spawn(
    "./build/e2e/m2h",
    [
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
  target = "/doc/r0/README.md",
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

  // Only the root holding the deep-linked document starts expanded; the
  // other root reads as a collapsed parallel row.
  await expect(page.getByRole("button", { name: "root-a" })).toHaveAttribute(
    "aria-expanded",
    "true",
  );
  await expect(page.getByRole("button", { name: "root-b" })).toHaveAttribute(
    "aria-expanded",
    "false",
  );
  await expect(page.locator(".markdown-body h1")).toHaveText("Root A Readme");
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r0/README.md",
  );

  // The file row visibly renders its icon and name: the context-menu refactor
  // once emptied the button while aria-label-based queries kept passing.
  const file = page.getByRole("button", {
    name: "Root A Readme，r0/README.md",
  });
  await expect(file).toContainText("README.md");
  await expect(file.locator("span.truncate")).toHaveText("README.md");
  await expect(file.locator(":scope > svg")).toHaveCount(1);

  // The second root's same-named README opens under its own virtual path.
  await page.getByRole("button", { name: "root-b" }).click();
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

test("rejected local links cannot navigate into a sibling root", async ({
  page,
  request,
}) => {
  await openWorkspace(page, "/doc/r0/README.md");

  const rejectedLink = page.getByRole("link", { name: "尝试跨到 Root B" });
  const href = "/__m2h_invalid_local_reference__?target=..%2Fr1%2FREADME.md";
  await expect(rejectedLink).toHaveAttribute("href", href);

  // The DOM address itself is inert, so ordinary clicks, modifier clicks,
  // middle-clicks and "open in new tab" all resolve to the same 404 endpoint
  // instead of relying on the React click handler for isolation.
  expect(
    await rejectedLink.evaluate((anchor) => new URL(anchor.href).pathname),
  ).toBe("/__m2h_invalid_local_reference__");
  const rejected = await request.get(baseURL + href);
  expect(rejected.status()).toBe(404);

  await rejectedLink.click();
  await page.waitForURL(`**/__m2h_invalid_local_reference__?**`);
  expect(new URL(page.url()).pathname).toBe("/__m2h_invalid_local_reference__");
  await expect(page.locator("body")).not.toContainText("Root B Readme");
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

test("filter matches a root's name and keeps the tree grouped by root", async ({
  page,
}) => {
  await openWorkspace(page);

  // Both roots hold two documents each.
  await expect(page.getByText("4 个 Markdown 文件")).toBeVisible();

  // Matching the second root's name surfaces every document under it and
  // nothing from the first root.
  const search = page.getByRole("searchbox", { name: "筛选文件" });
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

test("raw routes serve each root's original Markdown bytes", async ({
  request,
}) => {
  const alpha = await request.get(`${baseURL}/raw/r0/README.md`);
  expect(alpha.status()).toBe(200);
  expect(alpha.headers()["content-type"]).toContain("text/markdown");
  expect(await alpha.text()).toContain("# Root A Readme");

  const beta = await request.get(`${baseURL}/raw/r1/README.md`);
  expect(beta.status()).toBe(200);
  expect(await beta.text()).toContain("# Root B Readme");

  // A multi-root workspace only serves raw documents through a known root id.
  // (Traversal itself is covered by the Go handler tests: every spec-compliant
  // URL client — browser fetch and Playwright included — normalizes ".."
  // path segments away before a request is ever sent.)
  for (const target of ["/raw/README.md", "/raw/r9/README.md"]) {
    const response = await request.get(baseURL + target);
    expect(response.status(), target).toBe(404);
    expect(await response.text()).not.toContain("# Root");
  }
});

test("right-clicking same-named files copies each root's own addresses", async ({
  page,
}) => {
  await openWorkspace(page, "/doc/r0/README.md");

  // The first root's same-named README copies r0 addresses. Right-clicking
  // must not select it: the open document stays r0's README.
  await page
    .getByRole("button", { name: "Root A Readme，r0/README.md" })
    .click({ button: "right" });
  await page.getByRole("menuitem", { name: "复制文档网页链接" }).click();
  await expect(page.getByRole("status")).toHaveText("已复制文档链接");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
    `${baseURL}/doc/r0/README.md`,
  );

  // The second root's copy routes to r1 — the same relative path never leaks
  // across roots — while the open document and URL remain r0's. The root
  // starts collapsed (the open document lives in r0), so it opens first.
  await page.getByRole("button", { name: "root-b" }).click();
  await page
    .getByRole("button", { name: "Root B Readme，r1/README.md" })
    .click({ button: "right" });
  await page.getByRole("menuitem", { name: "复制 Markdown 链接" }).click();
  await expect(page.getByRole("status")).toHaveText("已复制 Markdown 链接");
  expect(await page.evaluate(() => navigator.clipboard.readText())).toBe(
    `${baseURL}/raw/r1/README.md`,
  );
  expect(await page.evaluate(() => window.location.pathname)).toBe(
    "/doc/r0/README.md",
  );
  await expect(page.locator(".markdown-body h1")).toHaveText("Root A Readme");
});

test("context-menu open-in-new-tab really opens a popup with the document", async ({
  page,
  context,
}) => {
  await openWorkspace(page, "/doc/r1/README.md");

  await page
    .getByRole("button", { name: "Root B Readme，r1/README.md" })
    .click({ button: "right" });
  const [popup] = await Promise.all([
    context.waitForEvent("page"),
    page.getByRole("menuitem", { name: "新页面打开" }).click(),
  ]);
  await popup.waitForFunction(
    () => document.querySelector(".reader-document h1") !== null,
  );
  await expect(popup.locator(".markdown-body h1")).toHaveText("Root B Readme");
  expect(new URL(popup.url()).pathname).toBe("/doc/r1/README.md");
});
