import { type ChildProcess, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { expect, type Page, test } from "@playwright/test";

import { stopServerProcess } from "./server-process";

// Authenticated-mode regression: a second m2h instance (`--auth local`) on
// its own port and data directory. The default unauthenticated server the
// other specs use stays untouched — both modes must keep working side by
// side, exactly like a deployment that turns auth on later.
const authPort = 8874;
const baseURL = `http://127.0.0.1:${authPort}`;

let authServer: ChildProcess | undefined;
let dataDir: string | undefined;

test.beforeAll(async () => {
  dataDir = mkdtempSync(join(tmpdir(), "m2h-auth-e2e-"));
  authServer = spawn(
    "./build/e2e/m2h",
    [
      "--no-open",
      "--host",
      "127.0.0.1",
      "--port",
      String(authPort),
      "--auth",
      "local",
      "--data-dir",
      dataDir,
      "web/e2e/docs",
    ],
    {
      cwd: join(import.meta.dirname, "..", ".."),
      env: {
        ...process.env,
        M2H_LOCAL_USERS_JSON: JSON.stringify({
          alice: "password-1",
          bob: "password-2",
        }),
      },
      stdio: "ignore",
    },
  );
  // Wait for the listener: poll the capability endpoint.
  const deadline = Date.now() + 30_000;
  for (;;) {
    try {
      const response = await fetch(`${baseURL}/api/config`);
      if (response.ok) {
        break;
      }
    } catch {
      // not listening yet
    }
    if (Date.now() > deadline) {
      throw new Error("authenticated m2h server never came up");
    }
    await new Promise((resolve) => {
      setTimeout(resolve, 200);
    });
  }
});

test.afterAll(async () => {
  await stopServerProcess(authServer);
  if (dataDir !== undefined) {
    rmSync(dataDir, { recursive: true, force: true });
  }
});

async function login(page: Page, username: string, password: string) {
  await page.goto(`${baseURL}/`);
  await page.getByLabel("用户名").fill(username);
  await page.getByLabel("密码").fill(password);
  await page.getByRole("button", { name: "登录" }).click();
  await page.waitForURL(`${baseURL}/`);
}

test.describe("authenticated mode", () => {
  test("anonymous visitors meet the login page, not the documents", async ({
    page,
  }) => {
    await page.goto(`${baseURL}/`);
    await expect(page.getByRole("heading", { name: "登录 m2h" })).toBeVisible();

    const raw = await page.request.get(`${baseURL}/raw/README.md`);
    expect(raw.status()).toBe(401);
    const search = await page.request.get(`${baseURL}/api/search?q=demo`);
    expect(search.status()).toBe(401);
  });

  test("wrong credentials fail uniformly and rate-limit", async ({ page }) => {
    await page.goto(`${baseURL}/`);
    await page.getByLabel("用户名").fill("alice");
    await page.getByLabel("密码").fill("wrong-password");
    await page.getByRole("button", { name: "登录" }).click();
    await expect(page.getByRole("alert")).toContainText("用户名或密码错误");

    // The unknown-user variant reads exactly the same — no account probe.
    await page.getByLabel("用户名").fill("whoever");
    await page.getByRole("button", { name: "登录" }).click();
    await expect(page.getByRole("alert")).toContainText("用户名或密码错误");
  });

  test("login opens the reader and survives reload", async ({ page }) => {
    await login(page, "alice", "password-1");
    await expect(page.getByRole("button", { name: "设置" })).toBeVisible();

    await page.reload();
    // The session cookie restored the reader — no login page.
    await expect(page.getByRole("button", { name: "设置" })).toBeVisible({
      timeout: 10_000,
    });
    const raw = await page.request.get(`${baseURL}/raw/README.md`);
    expect(raw.status()).toBe(200);
  });

  test("logout closes the reader", async ({ page }) => {
    await login(page, "alice", "password-1");
    await page.getByRole("button", { name: "设置" }).click();
    await page.getByRole("menuitem", { name: /退出登录/ }).click();
    await expect(page.getByRole("heading", { name: "登录 m2h" })).toBeVisible();
    const raw = await page.request.get(`${baseURL}/raw/README.md`);
    expect(raw.status()).toBe(401);
  });

  test("annotations: highlight, comment, reload, and delete", async ({
    page,
  }) => {
    await login(page, "alice", "password-1");
    await page.goto(`${baseURL}/doc/README.md`);
    // The body mounts asynchronously after the document API answers.
    await page.waitForSelector("article p");

    // Select the document's first paragraph text through the DOM. The
    // whole selection is re-made on every retry: the annotation index
    // settles only after the rich-content pass, and a body remount leaves
    // an older selection pointing at detached nodes.
    const selectFirstParagraph = () =>
      page.evaluate(() => {
        const article = document.querySelector("article");
        const paragraph = article?.querySelector("p");
        if (paragraph === null || paragraph.firstChild === null) {
          throw new Error("fixture document lacks a paragraph");
        }
        const range = document.createRange();
        range.selectNodeContents(paragraph.firstChild);
        const selection = window.getSelection();
        selection?.removeAllRanges();
        selection?.addRange(range);
        document.dispatchEvent(new Event("selectionchange"));
      });
    await expect(async () => {
      await selectFirstParagraph();
      await page
        .getByRole("toolbar", { name: "标注选中文本" })
        .waitFor({ timeout: 1_000 });
    }).toPass({ timeout: 15_000 });
    await page.getByRole("button", { name: "评论" }).click();
    await page.getByPlaceholder("添加评论…").fill("需要补充备份方案");
    await page.getByRole("button", { name: "保存" }).click();

    const point = await page.evaluate(() => {
      window.getSelection()?.removeAllRanges();
      const text = document.querySelector("article p")?.firstChild;
      if (!text) throw new Error("missing annotated text");
      const range = document.createRange();
      range.setStart(text, 0);
      range.setEnd(text, Math.min(4, text.textContent?.length ?? 0));
      const rect = range.getBoundingClientRect();
      return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
    });
    await expect(async () => {
      await page.mouse.move(point.x + 2, point.y);
      await page.mouse.move(point.x, point.y);
      await expect(page.getByRole("button", { name: "编辑评论" })).toBeVisible({
        timeout: 500,
      });
    }).toPass({ timeout: 10_000 });
    await page.getByRole("button", { name: "编辑评论" }).click();
    await page.getByPlaceholder("添加评论…").fill("已经补充备份方案");
    await page.getByRole("button", { name: "保存" }).click();
    await expect(page.locator(".annotation-hover-card")).toContainText(
      "已经补充备份方案",
    );
    await page.mouse.move(point.x, point.y);
    await page.mouse.click(point.x, point.y);
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toContainText("已经补充备份方案");
    await expect(
      page.getByRole("button", { name: "收起标注面板" }),
    ).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: "收起标注面板" }).click();
    await page.getByRole("button", { name: "打开标注面板" }).click();
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toContainText("已经补充备份方案");

    // Reload: the annotation lives in SQLite and comes back.
    await page.reload();
    await page.getByRole("button", { name: /打开标注面板/ }).click();
    await expect(page.getByText("已经补充备份方案")).toBeVisible({
      timeout: 10_000,
    });

    // The author can delete from the in-text hover card.
    await expect(async () => {
      await page.mouse.move(point.x + 2, point.y);
      await page.mouse.move(point.x, point.y);
      await expect(page.getByRole("button", { name: "删除标注" })).toBeVisible({
        timeout: 500,
      });
    }).toPass({ timeout: 10_000 });
    await page.getByRole("button", { name: "删除标注" }).click();
    await expect(page.getByText(/还没有标注/)).toBeVisible();
  });

  test("token settings offer finite and permanent lifetimes", async ({
    page,
  }) => {
    await login(page, "alice", "password-1");
    await page.getByRole("button", { name: "设置" }).click();
    await page.getByRole("menuitem", { name: "凭据管理" }).click();
    const dialog = page.getByRole("dialog");
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("有效期").locator("option")).toHaveCount(11);
    await dialog.getByLabel("令牌名称").fill("permanent-e2e");
    await dialog.getByLabel("有效期").selectOption("permanent");
    await dialog.getByRole("button", { name: "创建" }).click();
    await expect(dialog.getByText("permanent-e2e")).toBeVisible();
    await expect(dialog.getByText(/永久/).last()).toBeVisible();
    await dialog.getByRole("button", { name: "撤销 permanent-e2e" }).click();
    await dialog.getByRole("button", { name: "确认撤销" }).click();
  });

  test("outline and annotations share a fixed, resizable rail", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await login(page, "alice", "password-1");
    await page.goto(`${baseURL}/doc/toc-scroll.md`);
    await expect(
      page.getByRole("navigation", { name: "文档目录" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "打开标注面板" }).click();
    const panel = page.getByRole("complementary", { name: "标注面板" });
    const separator = page.getByRole("separator", {
      name: "调整目录与标注高度",
    });
    await expect(panel).toBeVisible();
    await expect(separator).toBeVisible();
    const top = await panel.evaluate(
      (element) => element.getBoundingClientRect().top,
    );
    await page.evaluate(() => window.scrollTo(0, 600));
    expect(
      await panel.evaluate((element) => element.getBoundingClientRect().top),
    ).toBe(top);
    await separator.focus();
    await page.keyboard.press("ArrowUp");
    await expect(separator).toHaveAttribute("aria-valuenow", "45");
    const box = await separator.boundingBox();
    if (!box) throw new Error("missing separator geometry");
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2 + 80);
    await page.mouse.up();
    expect(
      Number(await separator.getAttribute("aria-valuenow")),
    ).toBeGreaterThan(45);
    await page.getByRole("button", { name: "收起标注面板" }).click();
    await expect(panel).toHaveCount(0);
  });

  test("annotation toggle also closes the mobile panel", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page, "alice", "password-1");
    await page.goto(`${baseURL}/doc/README.md`);
    await page.getByRole("button", { name: "打开标注面板" }).click();
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "收起标注面板" }).click();
    await expect(
      page.getByRole("button", { name: "打开标注面板" }),
    ).toBeVisible();
  });

  test("bearer tokens search and read within their scope", async ({
    page,
    request,
  }) => {
    await login(page, "alice", "password-1");
    const cookies = await page.context().cookies(baseURL);
    const cookieHeader = cookies
      .map((cookie) => `${cookie.name}=${cookie.value}`)
      .join("; ");

    const created = await request.post(`${baseURL}/api/tokens`, {
      headers: { Cookie: cookieHeader, Origin: baseURL },
      data: {
        kind: "persistent",
        name: "e2e-agent",
        scope: {
          permissions: ["document:read", "search:read"],
          resource: { type: "all_published" },
        },
      },
    });
    expect(created.status()).toBe(201);
    const token = ((await created.json()) as { secret: string }).secret;

    const search = await request.get(`${baseURL}/api/search?q=demo`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(search.status()).toBe(200);

    const raw = await request.get(`${baseURL}/raw/README.md`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(raw.status()).toBe(200);

    // Token management stays session-only.
    const listed = await request.get(`${baseURL}/api/tokens`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(listed.status()).toBe(403);

    // Shared review comments are never part of document:read.
    const annotations = await request.get(
      `${baseURL}/api/annotations?path=README.md`,
      { headers: { Authorization: `Bearer ${token}` } },
    );
    expect(annotations.status()).toBe(403);
  });

  test("an unresolved annotation is re-anchored through the UI", async ({
    page,
    request,
  }) => {
    await login(page, "alice", "password-1");
    const cookies = await page.context().cookies(baseURL);
    const cookieHeader = cookies
      .map((cookie) => `${cookie.name}=${cookie.value}`)
      .join("; ");

    // Seed an annotation whose quote no longer exists in the document —
    // the state an edit behind the server's back leaves behind.
    const document = await request.get(
      `${baseURL}/api/document?path=README.md`,
      {
        headers: { Cookie: cookieHeader },
      },
    );
    const identity = (await document.json()) as {
      documentRef: { rootId: string; path: string };
      documentHash: string;
    };
    const created = await request.post(`${baseURL}/api/annotations`, {
      headers: { Cookie: cookieHeader, Origin: baseURL },
      data: {
        documentRef: identity.documentRef,
        documentHash: identity.documentHash,
        selector: {
          quote: { exact: "旧版本残留的句子", prefix: "", suffix: "" },
          position: { start: 0, end: 8 },
          version: 1,
        },
        comment: "位置失效的评审意见",
      },
    });
    expect(created.status()).toBe(201);

    await page.goto(`${baseURL}/doc/README.md`);
    await page.waitForSelector("article p");
    await page.getByRole("button", { name: /打开标注面板/ }).click();
    await expect(page.getByText("未定位")).toBeVisible({ timeout: 10_000 });

    // The human repair flow: re-associate onto freshly selected text.
    await page.getByRole("button", { name: "重新关联" }).click();
    await expect(page.getByText(/按 Esc 取消/)).toBeVisible();
    const selectFirstParagraph = () =>
      page.evaluate(() => {
        const article = document.querySelector("article");
        const paragraph = article?.querySelectorAll("p")[0];
        if (paragraph === null || paragraph.firstChild === null) {
          throw new Error("fixture document lacks a paragraph");
        }
        const range = document.createRange();
        range.selectNodeContents(paragraph.firstChild);
        const selection = window.getSelection();
        selection?.removeAllRanges();
        selection?.addRange(range);
        document.dispatchEvent(new Event("selectionchange"));
      });
    await expect(async () => {
      await selectFirstParagraph();
      await page
        .getByRole("toolbar", { name: "重新关联标注" })
        .waitFor({ timeout: 1_000 });
    }).toPass({ timeout: 15_000 });
    await page.getByRole("button", { name: "关联到此选区" }).click();

    // The annotation resolves: the badge and the flow hint disappear.
    await expect(page.getByText("未定位")).toBeHidden({ timeout: 10_000 });
    await expect(page.getByText(/按 Esc 取消/)).toBeHidden();

    // Reload proves the new anchor persisted.
    await page.reload();
    await page.getByRole("button", { name: /打开标注面板/ }).click();
    await expect(page.getByText("位置失效的评审意见")).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByText("未定位")).toBeHidden();
  });

  test("the re-anchor flow cancels on Escape with nothing selected", async ({
    page,
    request,
  }) => {
    await login(page, "alice", "password-1");
    const cookies = await page.context().cookies(baseURL);
    const cookieHeader = cookies
      .map((cookie) => `${cookie.name}=${cookie.value}`)
      .join("; ");
    const document = await request.get(
      `${baseURL}/api/document?path=README.md`,
      { headers: { Cookie: cookieHeader } },
    );
    const identity = (await document.json()) as {
      documentRef: { rootId: string; path: string };
      documentHash: string;
    };
    const created = await request.post(`${baseURL}/api/annotations`, {
      headers: { Cookie: cookieHeader, Origin: baseURL },
      data: {
        documentRef: identity.documentRef,
        documentHash: identity.documentHash,
        selector: {
          quote: { exact: "另一个失效引用", prefix: "", suffix: "" },
          position: { start: 0, end: 7 },
          version: 1,
        },
        comment: "待重新关联",
      },
    });
    expect(created.status()).toBe(201);

    await page.goto(`${baseURL}/doc/README.md`);
    await page.waitForSelector("article p");
    await page.getByRole("button", { name: /打开标注面板/ }).click();
    await expect(page.getByText("未定位")).toBeVisible({ timeout: 10_000 });

    // Enter the flow, select nothing, press Escape: the banner's promise.
    await page.getByRole("button", { name: "重新关联" }).click();
    await expect(page.getByText(/按 Esc 取消/)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByText(/按 Esc 取消/)).toBeHidden();
    // The annotation is untouched — still unresolved.
    await expect(page.getByText("未定位")).toBeVisible();
  });
});
