import { type ChildProcess, spawn } from "node:child_process";
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from "node:fs";
import os from "node:os";
import path from "node:path";
import { expect, test } from "@playwright/test";

// Real-browser coverage for the --hidden publishing switch. The suite builds
// one throwaway document tree with plain, hidden and protected fixtures and
// serves it twice — once with the default policy, once with --hidden — from
// the same binary the other suites build. The sidebar, the document routes,
// assets and search must move together; there is no front-end switch, the
// server alone decides the publishing scope. Protected paths (.git, .env)
// stay refused in both shapes.

const ports = { hidden: 8876, plain: 8877 };

// The suite owns the --hidden server, so page navigations must land there
// instead of the config's shared document server.
test.use({ baseURL: `http://127.0.0.1:${ports.hidden}` });

const servers: ChildProcess[] = [];
let root: string;

async function startServer(port: number, extraFlags: string[]): Promise<void> {
  const server = spawn(
    "./build/e2e/m2h",
    [
      ...extraFlags,
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
  servers.push(server);
  const baseURL = `http://127.0.0.1:${port}`;
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
      throw new Error(`hidden preview server :${port} did not become ready`);
    }
    await new Promise((resolve) => {
      setTimeout(resolve, 200);
    });
  }
}

test.beforeAll(async () => {
  root = mkdtempSync(path.join(os.tmpdir(), "m2h-hidden-"));
  writeFileSync(path.join(root, "README.md"), "# Root\n\nvisible text\n");
  writeFileSync(path.join(root, ".draft.md"), "# Draft\n\ndraft text\n");
  mkdirSync(path.join(root, ".notes"));
  writeFileSync(
    path.join(root, ".notes", "guide.md"),
    "# Guide\n\nnotes guide text\n",
  );
  writeFileSync(path.join(root, ".notes", "image.png"), "png");
  writeFileSync(path.join(root, ".env.production"), "TOKEN=1");
  await startServer(ports.hidden, ["--hidden"]);
  await startServer(ports.plain, []);
});

test.afterAll(() => {
  for (const server of servers) {
    server.kill("SIGTERM");
  }
  if (root) {
    rmSync(root, { recursive: true, force: true });
  }
});

test("the default shape hides dot-prefixed paths everywhere", async () => {
  const plain = `http://127.0.0.1:${ports.plain}`;

  const files = await fetch(`${plain}/api/files`);
  const payload = (await files.json()) as {
    roots: { files: { path: string }[] }[];
  };
  const paths = payload.roots[0].files.map((file) => file.path);
  expect(paths).toEqual(["README.md"]);

  expect((await fetch(`${plain}/api/document?path=.draft.md`)).status).toBe(
    404,
  );
  expect((await fetch(`${plain}/raw/.draft.md`)).status).toBe(404);
  expect((await fetch(`${plain}/assets/.notes/image.png`)).status).toBe(404);
  expect((await fetch(`${plain}/assets/.env.production`)).status).toBe(404);

  const search = await fetch(`${plain}/api/search?q=draft+text`);
  expect(((await search.json()) as { results: unknown[] }).results).toEqual([]);
});

test("--hidden surfaces hidden documents in the sidebar and opens them", async ({
  page,
}) => {
  await page.goto("/");
  const tree = page.locator('[aria-label^="Markdown 文件树"]');
  await expect(tree).toBeVisible();
  await expect(tree.getByText("README.md")).toBeVisible();
  await expect(tree.getByText(".draft.md")).toBeVisible();

  // A deep link into the hidden directory renders the document body.
  await page.goto("/doc/.notes/guide.md");
  await expect(page.locator(".markdown-body")).toContainText(
    "notes guide text",
  );

  // Every publishing entrance answers the same scope.
  expect(
    (await fetch(`http://127.0.0.1:${ports.hidden}/raw/.draft.md`)).status,
  ).toBe(200);
  expect(
    (await fetch(`http://127.0.0.1:${ports.hidden}/assets/.notes/image.png`))
      .status,
  ).toBe(200);
});

test("--hidden admits hidden documents to search, protected paths stay refused", async () => {
  const search = await fetch(
    `http://127.0.0.1:${ports.hidden}/api/search?q=draft+text`,
  );
  const payload = (await search.json()) as {
    results: { path: string }[];
  };
  expect(payload.results.map((result) => result.path)).toEqual([".draft.md"]);

  // Protection is not part of the switch: the derived env file stays 404
  // through both shapes.
  for (const port of [ports.hidden, ports.plain]) {
    expect(
      (await fetch(`http://127.0.0.1:${port}/assets/.env.production`)).status,
    ).toBe(404);
    expect(
      (
        await fetch(
          `http://127.0.0.1:${port}/api/document?path=.env.production`,
        )
      ).status,
    ).toBe(404);
  }
});
