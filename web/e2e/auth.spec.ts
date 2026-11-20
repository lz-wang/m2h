import { type ChildProcess, spawn } from "node:child_process";
import { mkdtempSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";

import { expect, type Locator, type Page, test } from "@playwright/test";

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

async function dragAnnotationHandle(
  page: Page,
  handle: Locator,
  dx: number,
  dy: number,
) {
  const rect = await handle.boundingBox();
  if (!rect) throw new Error("missing annotation window control");
  const x = rect.x + rect.width / 2;
  const y = rect.y + rect.height / 2;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + dx, y + dy, { steps: 5 });
  await page.mouse.up();
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

  test("logout returns to the system-themed login page", async ({ page }) => {
    await page.emulateMedia({ colorScheme: "dark" });
    await login(page, "alice", "password-1");
    await page.goto(`${baseURL}/doc/README.md?mode=light`);
    await expect(page.locator("html")).toHaveCSS("color-scheme", "light");
    await page.getByRole("button", { name: "设置" }).click();
    await page.getByRole("menuitem", { name: /退出登录/ }).click();
    await expect(page.getByRole("heading", { name: "登录 m2h" })).toBeVisible();
    const root = page.locator("html");
    const loginPage = page.getByRole("main");
    const darkClass = /(?:^|\s)dark(?:\s|$)/;
    await expect(root).toHaveAttribute("data-mode", "auto");
    await expect(root).toHaveClass(darkClass);
    const darkBackground = await loginPage.evaluate(
      (element) => getComputedStyle(element).backgroundColor,
    );
    await page.emulateMedia({ colorScheme: "light" });
    await expect(root).not.toHaveClass(darkClass);
    await expect(loginPage).not.toHaveCSS("background-color", darkBackground);
    await page.emulateMedia({ colorScheme: "dark" });
    await expect(loginPage).toHaveCSS("background-color", darkBackground);
    const raw = await page.request.get(`${baseURL}/raw/README.md`);
    expect(raw.status()).toBe(401);
  });

  test("annotations: highlight, comment, reload, and delete", async ({
    page,
  }) => {
    await page
      .context()
      .grantPermissions(["clipboard-read", "clipboard-write"], {
        origin: baseURL,
      });
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
    const selectedText = await page.evaluate(() =>
      window.getSelection()?.toString(),
    );
    await page.getByRole("button", { name: "评论" }).click();
    const draftRange = () =>
      page.evaluate(() =>
        Array.from(CSS.highlights.get("m2h-annotation-draft") ?? [], (range) =>
          range.toString(),
        ),
      );
    const composer = page.getByRole("region", { name: "新建评论" });
    const editor = composer.getByPlaceholder("添加评论…");
    await expect(editor).toBeFocused();
    await editor.fill("正在撰写");
    await expect.poll(draftRange).toEqual([selectedText]);
    const initialCard = await composer.boundingBox();
    const initialEditor = await editor.boundingBox();
    if (!initialCard || !initialEditor)
      throw new Error("missing comment composer");
    await dragAnnotationHandle(
      page,
      composer.getByRole("button", { name: "移动标注窗口" }),
      48,
      32,
    );
    expect(await composer.boundingBox()).toMatchObject({
      x: initialCard.x + 48,
      y: initialCard.y + 32,
    });
    await dragAnnotationHandle(
      page,
      composer.getByRole("button", { name: "从右下角调整标注窗口大小" }),
      60,
      44,
    );
    expect(await editor.boundingBox()).toMatchObject({
      width: initialEditor.width + 60,
      height: initialEditor.height + 44,
    });
    const cornerDrags = [
      {
        name: "左上角",
        cursor: "nwse-resize",
        dx: -20,
        dy: -12,
        originX: -20,
        originY: -12,
        width: 20,
        height: 12,
      },
      {
        name: "右上角",
        cursor: "nesw-resize",
        dx: 12,
        dy: 8,
        originX: 0,
        originY: 8,
        width: 12,
        height: -8,
      },
      {
        name: "左下角",
        cursor: "nesw-resize",
        dx: 16,
        dy: 10,
        originX: 16,
        originY: 0,
        width: -16,
        height: 10,
      },
    ];
    for (const {
      name,
      cursor,
      dx,
      dy,
      originX,
      originY,
      width,
      height,
    } of cornerDrags) {
      const handle = composer.getByRole("button", {
        name: `从${name}调整标注窗口大小`,
      });
      const before = await composer.boundingBox();
      if (!before) throw new Error("missing comment composer bounds");
      await expect(handle).toHaveCSS("cursor", cursor);
      await dragAnnotationHandle(page, handle, dx, dy);
      expect(await composer.boundingBox()).toMatchObject({
        x: before.x + originX,
        y: before.y + originY,
        width: before.width + width,
        height: before.height + height,
      });
    }
    await expect(editor).toBeFocused();
    await expect(editor).toHaveValue("正在撰写");
    await expect.poll(draftRange).toEqual([selectedText]);

    await page.getByRole("button", { name: "取消" }).click();
    await expect.poll(draftRange).toEqual([]);
    await expect
      .poll(() => page.evaluate(() => window.getSelection()?.toString()))
      .toBe(selectedText);
    await page.getByRole("button", { name: "评论" }).click();
    await page.getByPlaceholder("添加评论…").fill("需要补充备份方案");
    const draftPin = composer.getByRole("button", { name: "固定悬浮框" });
    await expect(draftPin).toHaveAttribute("aria-pressed", "false");
    await draftPin.click();
    await expect(draftPin.locator(".lucide-pin-off")).toBeVisible();
    const draftBounds = await composer.boundingBox();
    await page.getByRole("button", { name: "保存" }).click();
    await expect.poll(draftRange).toEqual([]);
    const savedCard = page.getByRole("region", { name: "标注窗口" });
    await expect(savedCard).toContainText("需要补充备份方案");
    await expect(
      savedCard.getByRole("button", { name: "固定悬浮框" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(await savedCard.boundingBox()).toEqual(draftBounds);
    await savedCard.getByRole("button", { name: "关闭标注窗口" }).click();

    const marker = page.locator(".annotation-comment-marker");
    await expect(marker.locator(".lucide-message-circle")).toHaveCount(1);
    const beforeScroll = await marker.boundingBox();
    if (!beforeScroll) throw new Error("missing comment marker");
    const hiddenOnScroll = await page.evaluate(() => {
      document.body.style.minHeight = "200vh";
      return new Promise<boolean>((resolve) => {
        const observer = new MutationObserver(() => {
          if (document.querySelector(".annotation-comment-marker")) return;
          observer.disconnect();
          resolve(true);
        });
        observer.observe(document.body, { childList: true, subtree: true });
        window.setTimeout(() => {
          observer.disconnect();
          resolve(false);
        }, 1000);
        window.scrollBy({ top: 36, behavior: "instant" });
      });
    });
    expect(hiddenOnScroll).toBe(true);
    await expect
      .poll(async () => (await marker.boundingBox())?.y)
      .toBeCloseTo(beforeScroll.y - 36);
    await page.evaluate(() => window.scrollTo({ top: 0, behavior: "instant" }));
    await expect
      .poll(async () => (await marker.boundingBox())?.y)
      .toBeCloseTo(beforeScroll.y);

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
    await expect(page.locator(".annotation-comment-box")).toHaveCSS(
      "min-height",
      "56px",
    );
    await page.getByRole("button", { name: "编辑评论", exact: true }).hover();
    await expect(page.getByRole("tooltip")).toHaveText("编辑评论");
    const actions = page.locator(".annotation-hover-actions");
    const positions = await actions.evaluate((element) => {
      const buttons = [...element.querySelectorAll("button")];
      return buttons.map((button) => ({
        text: button.getAttribute("aria-label"),
        left: button.getBoundingClientRect().left,
      }));
    });
    expect(positions.map((button) => button.text?.trim())).toEqual([
      "复制标注（Markdown）",
      "复制标注（JSON）",
      "编辑评论",
      "删除标注",
    ]);
    expect(positions[0].left).toBeLessThan(positions[1].left);
    await page.getByRole("button", { name: "复制标注（Markdown）" }).click();
    await expect(
      page
        .getByRole("button", { name: "复制标注（Markdown）" })
        .locator(".lucide-check"),
    ).toBeVisible();
    const copiedMarkdown = await page.evaluate(() =>
      navigator.clipboard.readText(),
    );
    expect(copiedMarkdown).toContain("# 文档评审意见");
    expect(copiedMarkdown).toContain("<!--评审意见--1--START-->");
    expect(copiedMarkdown).toContain("- 评审意见: 需要补充备份方案");
    expect(copiedMarkdown).toContain("需要补充备份方案");
    const match = copiedMarkdown.match(
      /Authorization: Bearer (m2h_[A-Za-z0-9_-]+)/,
    );
    expect(match).not.toBeNull();
    const token = match?.[1];
    if (!token) throw new Error("missing temporary credential");
    const allowed = await page.request.get(`${baseURL}/raw/README.md`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    const denied = await page.request.get(`${baseURL}/raw/scroll.md`, {
      headers: { Authorization: `Bearer ${token}` },
    });
    expect(allowed.status()).toBe(200);
    expect(denied.status()).toBe(403);
    await page.getByRole("button", { name: "复制标注（JSON）" }).click();
    await expect(
      page
        .getByRole("button", { name: "复制标注（JSON）" })
        .locator(".lucide-check"),
    ).toBeVisible();
    const singleExport = JSON.parse(
      await page.evaluate(() => navigator.clipboard.readText()),
    ) as { annotations: unknown[] };
    expect(singleExport.annotations).toHaveLength(1);
    await page.locator(".annotation-comment-box").dblclick();
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
    await page.getByRole("button", { name: "编辑此评论" }).click();
    await page.getByPlaceholder("添加评论…").fill("");
    await page.getByRole("button", { name: "保存" }).click();
    await expect(page.locator(".annotation-comment-marker")).toHaveCount(0);
    await page.getByRole("button", { name: "编辑此评论" }).click();
    await page.getByPlaceholder("添加评论…").fill("侧边栏已更新");
    await page.getByRole("button", { name: "保存" }).click();
    await expect(page.locator(".annotation-comment-marker")).toHaveCount(1);
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toContainText("侧边栏已更新");
    const annotationPanel = page.getByRole("complementary", {
      name: "标注面板",
    });
    const titleCenter = await annotationPanel
      .getByText(/本页标注/)
      .evaluate((element) => {
        const rect = element.getBoundingClientRect();
        return rect.top + rect.height / 2;
      });
    for (const name of [
      "复制全部标注（Markdown）",
      "复制全部标注（JSON）",
      "批量删除标注",
    ]) {
      const icon = await annotationPanel
        .getByRole("button", { name, exact: true })
        .locator("svg")
        .boundingBox();
      if (!icon) throw new Error(`missing annotation action: ${name}`);
      expect(Math.abs(icon.y + icon.height / 2 - titleCenter)).toBeLessThan(1);
    }
    await page.getByRole("button", { name: "复制全部标注（JSON）" }).click();
    await expect(
      page
        .getByRole("button", { name: "复制全部标注（JSON）" })
        .locator(".lucide-check"),
    ).toBeVisible();
    const exported = JSON.parse(
      await page.evaluate(() => navigator.clipboard.readText()),
    ) as {
      document: {
        url: string;
        authentication: { authorization: string; expiresInMinutes: number };
      };
      annotations: Array<{ comment: string; createdAt: string }>;
    };
    expect(exported.document.url).toBe(`${baseURL}/raw/README.md`);
    expect(exported.document.authentication.expiresInMinutes).toBe(30);
    expect(exported.document.authentication.authorization).toMatch(
      /^Bearer m2h_/,
    );
    expect(exported.annotations).toHaveLength(1);
    expect(exported.annotations[0].comment).toBe("侧边栏已更新");
    expect(exported.annotations[0].createdAt).toMatch(/T.*[+-]\d\d:\d\d$/);
    await expect(page.getByText(/已复制/)).toHaveCount(0);
    await page
      .getByRole("button", { name: "复制全部标注（Markdown）" })
      .click();
    await expect(
      page
        .getByRole("button", { name: "复制全部标注（Markdown）" })
        .locator(".lucide-check"),
    ).toBeVisible();
    expect(await page.evaluate(() => navigator.clipboard.readText())).toContain(
      "侧边栏已更新",
    );
    await page.getByRole("button", { name: "复制此标注（Markdown）" }).click();
    await expect(
      page
        .getByRole("button", { name: "复制此标注（Markdown）" })
        .locator(".lucide-check"),
    ).toBeVisible();
    await page.getByRole("button", { name: "复制此标注（JSON）" }).click();
    await expect(
      page
        .getByRole("button", { name: "复制此标注（JSON）" })
        .locator(".lucide-check"),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "收起标注面板" }),
    ).toHaveAttribute("aria-pressed", "true");

    await page.getByRole("button", { name: "收起标注面板" }).click();
    await page.getByRole("button", { name: "打开标注面板" }).click();
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toContainText("侧边栏已更新");

    // Reload restores both the annotation and the open panel from the URL.
    await expect(page).toHaveURL(`${baseURL}/doc/README.md?annotations=true`);
    await page.reload();
    await expect(page.getByText("侧边栏已更新")).toBeVisible({
      timeout: 10_000,
    });

    const card = page.getByRole("region", { name: "标注窗口" });
    const openHoverCard = () =>
      expect(async () => {
        await page.mouse.move(point.x + 2, point.y);
        await page.mouse.move(point.x, point.y);
        await expect(card).toBeVisible({ timeout: 500 });
      }).toPass({ timeout: 10_000 });
    await openHoverCard();

    const pin = card.getByRole("button", { name: "固定悬浮框" });
    await expect(pin).toHaveAttribute("aria-pressed", "false");
    await expect(pin.locator(".lucide-pin")).toBeVisible();
    await pin.click();
    await expect(pin).toHaveAttribute("aria-pressed", "true");
    await expect(pin.locator(".lucide-pin-off")).toBeVisible();
    await page.mouse.move(8, 8);
    await expect(card).toBeVisible();

    await pin.click();
    await expect(pin).toHaveAttribute("aria-pressed", "false");
    await expect(pin.locator(".lucide-pin")).toBeVisible();
    await page.mouse.move(8, 8);
    await expect(card).toBeHidden();
    await openHoverCard();

    const start = await card.boundingBox();
    if (!start) throw new Error("missing annotation hover card");
    await dragAnnotationHandle(
      page,
      card.getByRole("button", { name: "移动标注窗口" }),
      48,
      32,
    );
    expect(await card.boundingBox()).toMatchObject({
      x: start.x + 48,
      y: start.y + 32,
    });

    await dragAnnotationHandle(
      page,
      card.getByRole("button", { name: "从右下角调整标注窗口大小" }),
      60,
      44,
    );
    await page.mouse.move(8, 8);
    expect(await card.boundingBox()).toMatchObject({
      width: start.width + 60,
      height: start.height + 44,
    });
    await page.getByRole("button", { name: "删除标注", exact: true }).click();
    await expect(page.getByText(/还没有标注/)).toBeVisible();
  });

  test("settings and width menus close after choosing an option", async ({
    page,
  }) => {
    await login(page, "alice", "password-1");
    await page.getByRole("button", { name: "设置" }).click();
    const menu = page.getByRole("menu");
    await expect(menu.getByText("显示主题", { exact: true })).toHaveCount(0);
    await expect(menu.getByRole("menuitemradio")).toHaveText([
      "系统主题",
      "浅色主题",
      "深色主题",
    ]);
    await menu.getByRole("menuitemradio", { name: "深色主题" }).click();
    await expect(menu).toBeHidden();
    await page.getByRole("button", { name: /文档宽度/ }).click();
    await page.getByRole("menuitemradio", { name: "宽" }).click();
    await expect(page.getByRole("menu")).toBeHidden();
  });

  test("token settings offer finite and permanent lifetimes", async ({
    page,
  }) => {
    await login(page, "alice", "password-1");
    await page.getByRole("button", { name: "设置" }).click();
    await page.getByRole("menuitem", { name: "凭据管理" }).click();
    const dialog = page.getByRole("dialog", { name: "API 令牌", exact: true });
    await expect(dialog).toBeVisible();
    await dialog.getByRole("button", { name: "新建" }).click();
    const createDialog = page.getByRole("dialog", { name: "新建 API 令牌" });
    const mainBounds = await page
      .locator('[data-slot="dialog-content"]')
      .first()
      .boundingBox();
    if (!mainBounds) throw new Error("missing token manager geometry");
    const backdrop = await page.evaluate(
      ({ x, y }) => {
        const target = document.elementFromPoint(x, y);
        if (!target) return null;
        return {
          slot: target.getAttribute("data-slot"),
          blur: getComputedStyle(target).backdropFilter,
          pointerEvents: getComputedStyle(target).pointerEvents,
        };
      },
      { x: mainBounds.x + 20, y: mainBounds.y + 20 },
    );
    expect(backdrop?.slot).toBe("dialog-overlay");
    expect(backdrop?.blur).toContain("blur(");
    expect(backdrop?.pointerEvents).toBe("auto");
    await expect(
      createDialog.getByLabel("有效期").locator("option"),
    ).toHaveCount(11);
    await expect
      .poll(async () => {
        const popupBox = await createDialog.boundingBox();
        const viewport = page.viewportSize();
        if (!popupBox || !viewport) return Infinity;
        return Math.abs(popupBox.y + popupBox.height / 2 - viewport.height / 2);
      })
      .toBeLessThan(2);
    await createDialog.getByLabel("访问范围").selectOption("指定单篇文档");
    const rootSelect = createDialog.getByLabel("文档根目录");
    await expect(rootSelect.locator("option")).not.toHaveCount(0);
    const scopeBox = await createDialog.getByLabel("访问范围").boundingBox();
    const rootBox = await rootSelect.boundingBox();
    const documentBox = await createDialog
      .getByRole("combobox", { name: "文档路径" })
      .boundingBox();
    if (!scopeBox || !rootBox || !documentBox)
      throw new Error("missing scope geometry");
    expect(Math.abs(scopeBox.y - rootBox.y)).toBeLessThan(2);
    expect(
      Math.abs(documentBox.width - (rootBox.x + rootBox.width - scopeBox.x)),
    ).toBeLessThan(2);
    await createDialog.getByRole("combobox", { name: "文档路径" }).click();
    await page.getByRole("option", { name: "README.md", exact: true }).click();
    await expect(
      createDialog.getByRole("combobox", { name: "文档路径" }),
    ).toContainText("README.md");
    await createDialog.getByLabel("令牌名称").fill("permanent-e2e");
    await createDialog.getByLabel("有效期").selectOption("permanent");
    await createDialog.getByRole("button", { name: "创建" }).click();
    await expect(createDialog.getByText(/令牌已创建/)).toBeVisible();
    const closing = await createDialog.evaluate((element) => {
      const button = element.querySelector<HTMLButtonElement>(
        '[data-slot="dialog-close"]',
      );
      if (!button) throw new Error("missing close button");
      button.click();
      return {
        hasResult: element.textContent?.includes("令牌已创建"),
        hasForm: !!element.querySelector('input[aria-label="令牌名称"]'),
      };
    });
    expect(closing).toEqual({ hasResult: true, hasForm: false });
    await expect(createDialog).toBeHidden();
    const tokenRow = dialog.getByRole("row").filter({
      has: page.getByRole("cell", { name: "permanent-e2e", exact: true }),
    });
    await expect(
      tokenRow.getByRole("cell", { name: "单篇文档（README.md）" }),
    ).toBeVisible();
    await expect(
      tokenRow.getByRole("cell", { name: "永久", exact: true }),
    ).toBeVisible();
    await tokenRow.getByRole("button", { name: "删除 permanent-e2e" }).click();
    await expect(tokenRow).toHaveCount(0);
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
    await expect(page.locator(".reader-toc-slot")).toHaveCSS(
      "transition-duration",
      "0s",
    );
    const box = await separator.boundingBox();
    if (!box) throw new Error("missing separator geometry");
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2);
    await page.mouse.down();
    expect(
      await separator.evaluate((element) => element.hasPointerCapture(1)),
    ).toBe(true);
    await page.mouse.move(box.x + box.width / 2, box.y + box.height / 2 + 80, {
      steps: 5,
    });
    await page.mouse.up();
    await expect
      .poll(async () => Number(await separator.getAttribute("aria-valuenow")))
      .toBeGreaterThan(45);
    await page.getByRole("button", { name: "收起标注面板" }).click();
    await expect(panel).toHaveCount(0);
  });

  test("the right sidebar resizes with pointer and keyboard input and restores its width", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await login(page, "alice", "password-1");
    await page.goto(`${baseURL}/doc/toc-scroll.md`);
    const handle = page.getByRole("separator", { name: "调整右侧边栏宽度" });
    const rail = page.locator(".reader-rail");
    const slot = page.locator(".reader-rail-slot");
    const leftSidebar = page.locator('[data-slot="sidebar-container"]');
    const readerNavigation = page.getByRole("navigation", {
      name: "阅读位置导航",
    });
    await expect(handle).toBeVisible();
    await expect(rail).toHaveCSS("width", "240px");
    await expect(leftSidebar).toHaveCSS("width", "256px");
    await page.getByRole("button", { name: "打开标注面板" }).click();

    const leftHandle = page.getByRole("button", { name: "调整侧边栏宽度" });
    const dividerStyle = (target: Locator) =>
      target.evaluate((element) => {
        const { width, backgroundColor } = getComputedStyle(element, "::after");
        return { width, backgroundColor };
      });
    expect(await dividerStyle(handle)).toEqual(await dividerStyle(leftHandle));
    await leftHandle.hover();
    const hoveredDivider = await dividerStyle(leftHandle);
    await handle.hover();
    expect(await dividerStyle(handle)).toEqual(hoveredDivider);

    const box = await handle.boundingBox();
    if (!box) throw new Error("right sidebar handle is missing");
    await page.mouse.move(box.x + box.width / 2, box.y + 32);
    await page.mouse.down();
    await page.mouse.move(box.x + box.width / 2 - 80, box.y + 32, { steps: 5 });
    await expect(rail).toHaveCSS("width", "320px");
    await expect(slot).toHaveCSS("width", "320px");
    await expect(readerNavigation).toHaveCSS("right", "344px");
    expect(await dividerStyle(handle)).toEqual(hoveredDivider);
    await page.mouse.up();
    await expect(handle).toHaveAttribute("aria-valuenow", "320");

    await handle.press("ArrowLeft");
    await expect(handle).toHaveAttribute("aria-valuenow", "336");
    await expect(leftSidebar).toHaveCSS("width", "256px");
    await page.reload();
    await expect(handle).toHaveAttribute("aria-valuenow", "336");
    await expect(rail).toHaveCSS("width", "336px");

    await page.setViewportSize({ width: 375, height: 780 });
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toBeVisible();
    await expect(rail).toBeVisible();
    await expect(handle).toHaveCount(0);
  });

  test("home hides both panels and document history restores the annotation panel", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 900 });
    await login(page, "alice", "password-1");
    await page.goto(`${baseURL}/doc/toc-scroll.md`);
    const outline = page.getByRole("navigation", { name: "文档目录" });
    const panel = page.getByRole("complementary", { name: "标注面板" });
    await expect(outline).toBeVisible();
    await page.getByRole("button", { name: "打开标注面板" }).click();
    await expect(panel).toBeVisible();
    await page.getByRole("button", { name: "文档首页" }).click();

    await expect(page.getByRole("article", { name: "文档首页" })).toBeVisible();
    await expect(page.locator(".markdown-body")).toHaveCount(0);
    await expect(outline).toHaveCount(0);
    await expect(panel).toHaveCount(0);
    await expect(page.getByRole("button", { name: /标注面板/ })).toHaveCount(0);
    await expect(page).toHaveURL(`${baseURL}/`);
    await page.goBack();
    await expect(outline).toBeVisible();
    await expect(panel).toBeVisible();
    await expect(page).toHaveURL(/annotations=true/);
    await page.goForward();
    await expect(panel).toBeHidden();
    await expect(page).toHaveURL(`${baseURL}/`);
  });

  test("annotation links open the mobile panel and closing preserves other URL settings", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(
      `${baseURL}/doc/README.md?mode=light&toc=false&annotations=true`,
    );
    await page.getByLabel("用户名").fill("alice");
    await page.getByLabel("密码").fill("password-1");
    await page.getByRole("button", { name: "登录" }).click();
    await expect(
      page.getByRole("complementary", { name: "标注面板" }),
    ).toBeVisible();
    await page.getByRole("button", { name: "收起标注面板" }).click();
    await expect(page).toHaveURL(
      `${baseURL}/doc/README.md?mode=light&toc=false`,
    );
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
    const createOrphan = (comment: string) =>
      request.post(`${baseURL}/api/annotations`, {
        headers: { Cookie: cookieHeader, Origin: baseURL },
        data: {
          documentRef: identity.documentRef,
          documentHash: identity.documentHash,
          selector: {
            quote: { exact: "旧版本残留的句子", prefix: "", suffix: "" },
            position: { start: 0, end: 8 },
            version: 1,
          },
          comment,
        },
      });
    expect((await createOrphan("位置失效的评审意见")).status()).toBe(201);

    await page.goto(`${baseURL}/doc/README.md`);
    await page.waitForSelector("article p");
    await page.getByRole("button", { name: /打开标注面板/ }).click();
    const locate = page
      .getByRole("listitem")
      .filter({ hasText: "旧版本残留的句子" })
      .getByRole("button", { name: "定位到正文" });
    await expect(locate).toBeDisabled({ timeout: 10_000 });

    await locate.hover();
    await expect(page.getByRole("tooltip")).toHaveText(
      "此标注无法定位，文档可能已经更新",
    );

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

    // The annotation resolves and the locate control becomes available.
    await expect(
      page
        .getByRole("listitem")
        .filter({ hasText: "位置失效的评审意见" })
        .getByRole("button", { name: "定位到正文" }),
    ).toBeEnabled({ timeout: 10_000 });
    await expect(page.getByText(/按 Esc 取消/)).toBeHidden();

    // Reload proves the new anchor persisted.
    await page.reload();
    await expect(page.getByText("位置失效的评审意见")).toBeVisible({
      timeout: 10_000,
    });
    await expect(page.getByRole("button", { name: "重新关联" })).toBeHidden();

    expect((await createOrphan("确认前新增的失效标注")).status()).toBe(201);
    await page.reload();
    await expect(page.getByRole("button", { name: "重新关联" })).toBeVisible();
    await page.getByRole("button", { name: "删除此标注" }).first().hover();
    await expect(
      page.getByRole("tooltip").filter({ hasText: /^删除此标注$/ }),
    ).toBeVisible();

    const bulkDelete = page.getByRole("button", { name: "批量删除标注" });
    await bulkDelete.hover();
    await expect(
      page.getByRole("tooltip").filter({ hasText: /^批量删除标注$/ }),
    ).toBeVisible();
    await bulkDelete.click();
    const dialog = page.getByRole("dialog", { name: "批量删除标注" });
    await expect
      .poll(() =>
        dialog.evaluate((element) => {
          const rect = element.getBoundingClientRect();
          return Math.hypot(
            rect.left + rect.width / 2 - window.innerWidth / 2,
            rect.top + rect.height / 2 - window.innerHeight / 2,
          );
        }),
      )
      .toBeLessThan(1);
    await page.getByRole("button", { name: "确认删除" }).click();
    await expect(page.getByText("确认前新增的失效标注")).toBeHidden();
    await expect(page.getByText("位置失效的评审意见")).toBeVisible();

    await page.getByRole("button", { name: "批量删除标注" }).click();
    await expect(page.getByRole("button", { name: "确认删除" })).toBeDisabled();
    await page
      .getByRole("checkbox", { name: "删除我在当前文档中的全部标注" })
      .check();
    await page.getByRole("button", { name: "确认删除" }).click();
    await expect(page.getByText(/还没有标注/)).toBeVisible();
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
    const locate = page
      .getByRole("listitem")
      .filter({ hasText: "待重新关联" })
      .getByRole("button", { name: "定位到正文" });
    await expect(locate).toBeDisabled({ timeout: 10_000 });

    // Enter the flow, select nothing, press Escape: the banner's promise.
    await page.getByRole("button", { name: "重新关联" }).click();
    await expect(page.getByText(/按 Esc 取消/)).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(page.getByText(/按 Esc 取消/)).toBeHidden();
    // The annotation is untouched — still unresolved.
    await expect(locate).toBeDisabled();

    await page.getByRole("button", { name: "删除此标注" }).click();
    await expect(page.getByText("待重新关联")).toBeHidden();
  });
  test("annotation deletion keeps errors visible and retries only failed items", async ({
    page,
  }) => {
    await login(page, "alice", "password-1");
    const path = "README.md";
    const identity = await (
      await page.request.get(`${baseURL}/api/document?path=${path}`)
    ).json();
    const seed = async (exact: string, comment: string): Promise<string> => {
      const response = await page.request.post(`${baseURL}/api/annotations`, {
        headers: { Origin: baseURL },
        data: {
          documentRef: identity.documentRef,
          documentHash: identity.documentHash,
          selector: {
            quote: { exact, prefix: "", suffix: "" },
            position: { start: 0, end: exact.length },
            version: 1,
          },
          comment,
        },
      });
      expect(response.status()).toBe(201);
      return (await response.json()).id;
    };
    const popupId = await seed("此文档", "悬浮框删除失败");
    const rowId = await seed("无匹配文本 A", "侧栏删除失败");
    const bulkOk = await seed("无匹配文本 B", "批量删除成功");
    const bulkFailed = await seed("无匹配文本 C", "批量删除失败");
    const firstFailure = new Map([
      [popupId, 500],
      [rowId, 403],
      [bulkFailed, 500],
    ]);
    const attempts = new Map<string, number>();
    await page.route("**/api/annotations/*", async (route) => {
      if (route.request().method() === "DELETE") {
        const id =
          new URL(route.request().url()).pathname.split("/").at(-1) ?? "";
        const count = (attempts.get(id) ?? 0) + 1;
        attempts.set(id, count);
        const status = firstFailure.get(id);
        if (count === 1 && status !== undefined) {
          await route.fulfill({
            status,
            json: { error: "simulated delete failure" },
          });
          return;
        }
      }
      await route.continue();
    });
    await page.goto(`${baseURL}/doc/${path}`);
    const paragraph = page.locator("article p").first();
    const point = await paragraph.evaluate((element) => {
      const text = element.firstChild;
      if (!text) throw new Error("missing annotated text");
      const range = document.createRange();
      range.setStart(text, 0);
      range.setEnd(text, 1);
      const rect = range.getBoundingClientRect();
      return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
    });
    const card = page.getByRole("region", { name: "标注窗口" });
    await expect(async () => {
      await page.mouse.move(point.x + 1, point.y);
      await page.mouse.move(point.x, point.y);
      await expect(card).toBeVisible({ timeout: 500 });
    }).toPass({ timeout: 10_000 });
    await card.getByRole("button", { name: "删除标注", exact: true }).click();
    await expect(card.getByRole("alert")).toHaveText("删除标注失败，请重试。");
    await page.mouse.move(8, 8);
    await expect(card).toBeVisible();
    await card.getByRole("button", { name: "重试删除标注" }).click();
    await expect(card).toBeHidden();

    await page.getByRole("button", { name: "打开标注面板" }).click();
    const sidebar = page.getByRole("complementary", { name: "标注面板" });
    const row = sidebar
      .getByRole("listitem")
      .filter({ hasText: "侧栏删除失败" });
    await row.getByRole("button", { name: "删除此标注" }).click();
    await expect(row.getByRole("alert")).toHaveText("没有权限执行此操作。");
    await row.getByRole("button", { name: "重试删除此标注" }).click();
    await expect(row).toBeHidden();

    await sidebar.getByRole("button", { name: "批量删除标注" }).click();
    const dialog = page.getByRole("dialog", { name: "批量删除标注" });
    await expect(
      dialog.getByRole("checkbox", { name: "删除我在当前文档中的全部标注" }),
    ).toBeVisible();
    await dialog.getByRole("button", { name: "确认删除" }).click();
    await expect(dialog.getByRole("alert")).toContainText(
      "已删除 1 条，1 条删除失败。",
    );
    await expect(dialog.getByRole("alert")).toContainText(
      "删除标注失败，请重试。",
    );
    const partial = await page.request.get(
      `${baseURL}/api/annotations?path=${path}`,
    );
    const remaining = (await partial.json()) as {
      annotations: Array<{ id: string }>;
    };
    expect(remaining.annotations.map(({ id }) => id)).toEqual([bulkFailed]);
    await dialog.getByRole("button", { name: "重试失败项" }).click();
    await expect(dialog).toBeHidden();
    await expect(sidebar.getByText(/还没有标注/)).toBeVisible();
    expect(attempts.get(popupId)).toBe(2);
    expect(attempts.get(rowId)).toBe(2);
    expect(attempts.get(bulkOk)).toBe(1);
    expect(attempts.get(bulkFailed)).toBe(2);
  });

  test("overlapping annotations offer the exact hits and independent actions", async ({
    page,
  }) => {
    await page
      .context()
      .grantPermissions(["clipboard-read", "clipboard-write"], {
        origin: baseURL,
      });
    await login(page, "alice", "password-1");
    const documentResponse = await page.request.get(
      `${baseURL}/api/document?path=README.md`,
    );
    const identity = (await documentResponse.json()) as {
      documentRef: { rootId: string; path: string };
      documentHash: string;
    };
    const create = async (exact: string, comment: string) => {
      const response = await page.request.post(`${baseURL}/api/annotations`, {
        headers: { Origin: baseURL },
        data: {
          documentRef: identity.documentRef,
          documentHash: identity.documentHash,
          selector: {
            quote: { exact, prefix: "", suffix: "" },
            position: { start: 0, end: exact.length },
            version: 1,
          },
          comment,
        },
      });
      expect(response.status()).toBe(201);
      return (await response.json()) as { id: string; updatedAt: number };
    };
    const wide = await create("此文档用于验证正文渲染", "较宽选区的标注");
    const older = await create("验证正文", "同一选区的旧标注");
    const newer = await create("验证正文", "同一选区的新标注");

    await page.goto(`${baseURL}/doc/README.md`);
    await page.waitForSelector("article p");
    const pointAt = (offset: number) =>
      page.evaluate((offset) => {
        const text = document.querySelector("article p")?.firstChild;
        if (!text) throw new Error("missing annotated text");
        const range = document.createRange();
        range.setStart(text, offset);
        range.setEnd(text, offset + 1);
        const rect = range.getBoundingClientRect();
        return { x: rect.left + rect.width / 2, y: rect.top + rect.height / 2 };
      }, offset);
    const wideOnly = await pointAt(1);
    const overlap = await pointAt(6);
    const card = page.getByRole("region", { name: "标注窗口" });
    const list = card.getByRole("list", { name: "重叠标注" });
    const comment = card.locator(".annotation-comment-box");

    await expect(async () => {
      await page.mouse.move(wideOnly.x + 1, wideOnly.y);
      await page.mouse.move(wideOnly.x, wideOnly.y);
      await expect(comment).toHaveText("较宽选区的标注", { timeout: 500 });
    }).toPass({ timeout: 10_000 });
    await expect(list).toBeHidden();
    await expect(card.getByText("alice", { exact: true })).toBeVisible();
    await expect(card.locator("time")).toHaveAttribute(
      "datetime",
      new Date(wide.updatedAt).toISOString(),
    );

    const wideCard = card.filter({
      has: page
        .locator(".annotation-comment-box")
        .filter({ hasText: "较宽选区的标注" }),
    });
    await wideCard.getByRole("button", { name: "固定悬浮框" }).click();
    await dragAnnotationHandle(
      page,
      wideCard.getByRole("button", { name: "移动标注窗口" }),
      -180,
      220,
    );
    await page.mouse.move(overlap.x, overlap.y);
    const overlapCard = card.filter({
      has: page.getByRole("list", { name: "重叠标注" }),
    });
    await expect(overlapCard.getByText("3 条标注")).toBeVisible();
    await expect(wideCard).toBeVisible();
    await page.mouse.click(overlap.x, overlap.y);
    await expect(
      overlapCard.getByRole("button", { name: "固定悬浮框" }),
    ).toHaveAttribute("aria-pressed", "true");
    await wideCard.getByRole("button", { name: "关闭标注窗口" }).click();
    await expect(card).toHaveCount(1);
    await page.mouse.move(8, 8);
    await expect(list.getByRole("button")).toHaveText([
      /同一选区的新标注/,
      /同一选区的旧标注/,
      /较宽选区的标注/,
    ]);

    await list.getByRole("button", { name: /同一选区的新标注/ }).click();
    await expect(comment).toHaveText("同一选区的新标注");
    await expect(card.locator("time")).toHaveAttribute(
      "datetime",
      new Date(newer.updatedAt).toISOString(),
    );
    const copy = card.getByRole("button", { name: "复制标注（JSON）" });
    await copy.click();
    await expect(copy.locator(".lucide-check")).toBeVisible();
    const copied = JSON.parse(
      await page.evaluate(() => navigator.clipboard.readText()),
    );
    expect(copied.annotations).toEqual([
      expect.objectContaining({
        id: newer.id,
        comment: "同一选区的新标注",
      }),
    ]);

    await page.getByRole("button", { name: "打开标注面板" }).click();
    const sidebar = page.getByRole("complementary", { name: "标注面板" });
    const sidebarTime = sidebar.locator(
      `[data-annotation-id="${newer.id}"] time`,
    );
    await expect(sidebarTime).toHaveAttribute(
      "datetime",
      new Date(newer.updatedAt).toISOString(),
    );

    await card.getByRole("button", { name: "编辑评论", exact: true }).click();
    await card.getByPlaceholder("添加评论…").fill("更新后的新标注");
    const updateResponse = page.waitForResponse(
      (response) =>
        response.url() === `${baseURL}/api/annotations/${newer.id}/comment` &&
        response.request().method() === "PATCH",
    );
    await card.getByRole("button", { name: "保存" }).click();
    const { updatedAt } = (await (await updateResponse).json()) as {
      updatedAt: number;
    };
    expect(updatedAt).toBeGreaterThan(newer.updatedAt);
    const updatedTime = new Date(updatedAt).toISOString();
    await expect(comment).toHaveText("更新后的新标注");
    await expect(card.locator("time")).toHaveAttribute("datetime", updatedTime);
    await expect(sidebarTime).toHaveAttribute("datetime", updatedTime);
    await expect(card).not.toContainText("更新于");
    await expect(sidebar).not.toContainText("更新于");
    await card.getByRole("button", { name: "返回标注列表" }).click();
    await expect(
      list.getByRole("button", { name: /更新后的新标注/ }).locator("time"),
    ).toHaveAttribute("datetime", updatedTime);
    await list.getByRole("button", { name: /同一选区的旧标注/ }).click();
    await card.getByRole("button", { name: "删除标注", exact: true }).click();
    await expect(list.getByRole("button")).toHaveText([
      /更新后的新标注/,
      /较宽选区的标注/,
    ]);

    const remaining = await page.request.get(
      `${baseURL}/api/annotations?path=README.md`,
    );
    const { annotations: entries } = (await remaining.json()) as {
      annotations: Array<{ id: string }>;
    };
    expect(entries.map(({ id }) => id)).toEqual([wide.id, newer.id]);
    expect(entries.map(({ id }) => id)).not.toContain(older.id);
  });
});
