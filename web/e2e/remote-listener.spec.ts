import { type ChildProcess, spawn } from "node:child_process";
import path from "node:path";
import { expect, test } from "@playwright/test";

// Real-browser regression for the VPS shape: the server binds a non-loopback
// listener (--host 0.0.0.0) and the browser reaches it through 127.0.0.1. The
// config's shared server binds 127.0.0.1, so this spec starts its own
// `m2h --host 0.0.0.0` process on another port once that build exists — same
// binary, exactly the command a VPS deployment types. Serving documents must
// not depend on loopback-only conveniences. The server's local paths are not
// conditional anymore — they never enter the API on any listener — so this
// spec now locks down that unconditional contract on the public deployment
// shape and keeps the local-path affordances out of the UI.

const port = 8875;
const baseURL = `http://127.0.0.1:${port}`;

let server: ChildProcess | null = null;

test.beforeAll(async () => {
  server = spawn(
    "./build/e2e/m2h",
    ["--no-open", "--host", "0.0.0.0", "--port", String(port), "web/e2e/docs"],
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
      throw new Error("non-loopback preview server did not become ready");
    }
    await new Promise((resolve) => {
      setTimeout(resolve, 200);
    });
  }
});

test.afterAll(() => {
  server?.kill("SIGTERM");
});

async function openDocument(
  page: import("@playwright/test").Page,
  target = "/doc/images.md",
) {
  await page.goto(baseURL + target);
  await page.waitForFunction(
    () =>
      document.querySelector(".markdown-body h1, .markdown-body p") !== null,
  );
}

test("serves documents and the file tree with a path-free API", async ({
  page,
  request,
}) => {
  // The API is the contract: no root summary carries the serving machine's
  // paths, separators or input kind, and the envelope carries no server-picked
  // default document — on this public listener or any other.
  const response = await request.get(`${baseURL}/api/files`);
  const listing = (await response.json()) as {
    roots: Array<Record<string, unknown>>;
    defaultDocument?: unknown;
  };
  expect(listing.roots).toHaveLength(1);
  expect(Object.keys(listing.roots[0] ?? {}).sort()).toEqual([
    "files",
    "id",
    "name",
  ]);
  expect(listing.defaultDocument).toBeUndefined();

  // Documents and the file tree still render through the loopback-facing URL.
  await openDocument(page);
  await expect(page.locator(".markdown-body h1")).toHaveText(
    "延迟图片回归文档",
  );
  await expect(
    page.getByRole("button", { name: "延迟图片回归文档，images.md" }),
  ).toBeVisible();
});

test("serves the health endpoint with hardened headers", async ({
  request,
}) => {
  // The real binary path: the non-loopback server must answer /healthz
  // without touching documents, and the response carries the same
  // hardening baseline as every other route.
  const response = await request.get(`${baseURL}/healthz`);
  expect(response.status()).toBe(200);
  expect(await response.text()).toBe("ok\n");
  expect(response.headers()["content-type"]).toBe("text/plain; charset=utf-8");
  expect(response.headers()["cache-control"]).toBe("no-store");
  expect(response.headers()["x-content-type-options"]).toBe("nosniff");
  expect(response.headers()["content-security-policy"]).toContain(
    "script-src 'self'",
  );
});

test("keeps every server-local path affordance out of the UI", async ({
  page,
}) => {
  await openDocument(page);

  // The toolbar share menu keeps the shareable identities only.
  await page.getByRole("button", { name: "分享文档" }).click();
  await expect(
    page.getByRole("menuitem", { name: "复制文档网页链接" }),
  ).toBeVisible();
  await expect(
    page.getByRole("menuitem", { name: "复制文档本地路径" }),
  ).toHaveCount(0);
  await page.keyboard.press("Escape");

  // The file context menu keeps its open/copy actions only.
  await page
    .getByRole("button", { name: "延迟图片回归文档，images.md" })
    .click({ button: "right" });
  await expect(
    page.getByRole("menuitem", { name: "新页面打开" }),
  ).toBeVisible();
  await expect(
    page.getByRole("menuitem", { name: "复制文档本地路径" }),
  ).toHaveCount(0);
  await page.keyboard.press("Escape");

  // Directory rows carry no context menu and no absolute-path tooltip. Hover
  // first and let the tooltip delay elapse: asserting absence before a tooltip
  // could even open would prove nothing.
  const directory = page.getByRole("button", { name: "tree", exact: true });
  await directory.click({ button: "right" });
  await expect(page.getByRole("menuitem")).toHaveCount(0);
  await directory.hover();
  await page.waitForTimeout(800);
  await expect(page.locator(".tree-tooltip-path")).toHaveCount(0);
});
