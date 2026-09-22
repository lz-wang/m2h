import { type ChildProcess, spawn } from "node:child_process";
import { mkdtempSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { expect, test } from "@playwright/test";

// Real-browser coverage for the .gitignore publishing filter. This spec
// builds a throwaway document tree with its own rule files, serves it with a
// dedicated `m2h` process (same binary the other suites build), and rewrites
// the rules mid-run: a fresh API request must honor the new rules at once,
// and the sidebar catches up after a reload — no watcher, by design.

const port = 8875;
const baseURL = `http://127.0.0.1:${port}`;

// The suite owns its server, so every page navigation must land there
// instead of the config's shared document server.
test.use({ baseURL });

let server: ChildProcess | null = null;
let root: string;

async function startServer(): Promise<void> {
  server = spawn(
    "./build/e2e/m2h",
    [
      "--no-open",
      "--host",
      "127.0.0.1",
      "--port",
      String(port),
      root,
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
        return;
      }
    } catch {
      // Not listening yet — retry until the deadline.
    }
    if (Date.now() > deadline) {
      throw new Error("gitignore preview server did not become ready");
    }
    await new Promise((resolve) => {
      setTimeout(resolve, 200);
    });
  }
}

function writeRules(content: string): void {
  writeFileSync(path.join(root, ".gitignore"), content);
}

async function listedFiles(): Promise<string[]> {
  const response = await fetch(`${baseURL}/api/files`);
  const payload = (await response.json()) as {
    roots: { files: { path: string }[] }[];
  };
  return payload.roots[0].files.map((file) => file.path);
}

test.beforeAll(async () => {
  root = mkdtempSync(path.join(os.tmpdir(), "m2h-gitignore-"));
  writeFileSync(path.join(root, "visible.md"), "# Visible\n\nstay put\n");
  writeFileSync(path.join(root, "hidden.md"), "# Hidden\n\nvanish\n");
  writeFileSync(path.join(root, "picture.png"), "png");
  await startServer();
});

test.afterAll(() => {
  server?.kill("SIGTERM");
  if (root) {
    rmSync(root, { recursive: true, force: true });
  }
});

test("rewriting the rules updates the next API answer at once", async () => {
  writeRules("");
  expect(await listedFiles()).toEqual(["hidden.md", "visible.md"]);

  writeRules("hidden.md\n");
  expect(await listedFiles()).toEqual(["visible.md"]);

  // A document route answers 404 for the ignored file the moment the rule
  // lands; the asset route refuses the same way.
  const document = await fetch(
    `${baseURL}/api/document?path=hidden.md`,
  );
  expect(document.status).toBe(404);
  const asset = await fetch(`${baseURL}/assets/picture.png`);
  expect(asset.status).toBe(200);
  writeRules("picture.png\n");
  const refused = await fetch(`${baseURL}/assets/picture.png`);
  expect(refused.status).toBe(404);

  // --no-gitignore is the off switch: a fresh process serves everything.
  // Covered in the CLI unit tests; here the rules go away entirely and the
  // workspace returns to its unfiltered shape.
  writeRules("");
  expect(await listedFiles()).toEqual(["hidden.md", "visible.md"]);
});

test("the sidebar catches up after a reload, not before", async ({
  page,
}) => {
  writeRules("");
  await page.goto("/");
  const tree = page.locator('[aria-label^="Markdown 文件树"]');
  await expect(tree).toBeVisible();
  await expect(tree.getByText("hidden.md")).toBeVisible();
  await expect(tree.getByText("visible.md")).toBeVisible();

  writeRules("hidden.md\n");
  await page.reload();
  await expect(tree).toBeVisible();
  await expect(tree.getByText("hidden.md")).toHaveCount(0);
  await expect(tree.getByText("visible.md")).toBeVisible();
});

test("search never surfaces an ignored document", async ({ page }) => {
  writeRules("");
  await page.goto("/doc/visible.md");
  await expect(page.locator(".markdown-body")).toContainText("stay put");

  writeRules("visible.md\n");
  const response = await page.request.get("/api/search?q=stay+put");
  expect(response.status()).toBe(200);
  const payload = (await response.json()) as { results: { path: string }[] };
  expect(payload.results).toEqual([]);
});
