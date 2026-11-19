import { expect, test } from "@playwright/test";

test("directory home opens document links and survives history navigation and reload", async ({
  page,
}) => {
  const requests: string[] = [];
  page.on("request", (request) => {
    requests.push(new URL(request.url()).pathname);
  });

  await page.goto("/");
  const title = page.getByRole("heading", { level: 1, name: "文档首页" });
  const created = page.getByRole("region", { name: "最新创建" });
  const updated = page.getByRole("region", { name: "最近更新" });
  await expect(title).toBeVisible();
  await expect(page).toHaveURL(/\/$/);
  await expect(created.getByRole("link")).toHaveCount(10);
  await expect(updated.getByRole("link")).toHaveCount(10);
  expect(requests.filter((path) => path === "/api/document")).toEqual([]);
  expect(requests.filter((path) => path === "/api/recent")).toHaveLength(1);

  const recent = updated.getByRole("link").first();
  const href = await recent.getAttribute("href");
  if (href === null) throw new Error("recent document link has no href");
  await recent.click();
  await expect(page).toHaveURL(new URL(href, page.url()).href);
  await expect(page.locator(".markdown-body")).toBeVisible();

  await page.goBack();
  await expect(title).toBeVisible();
  await expect(updated.getByRole("link")).toHaveCount(10);
  await expect(page.locator(".markdown-body")).toHaveCount(0);
  await page.goForward();
  await expect(page).toHaveURL(new URL(href, page.url()).href);
  await expect(page.locator(".markdown-body")).toBeVisible();

  await page.getByRole("button", { name: "文档首页" }).click();
  await expect(page).toHaveURL(/\/$/);
  await expect(created.getByRole("link")).toHaveCount(10);
  const beforeReload = requests.length;
  await page.reload();
  await expect(created.getByRole("link")).toHaveCount(10);
  expect(requests.slice(beforeReload)).toEqual(
    expect.arrayContaining(["/api/files", "/api/recent"]),
  );

  const beforeDeepLink = requests.length;
  await page.goto("/doc/README.md");
  await expect(page.locator(".markdown-body h1")).toHaveText("工作区入口文档");
  expect(requests.slice(beforeDeepLink)).not.toContain("/api/recent");
});

test("home lists remain readable on a narrow dark viewport", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/?mode=dark");
  await expect(page.locator("html")).toHaveClass(/dark/);
  const created = page.getByRole("region", { name: "最新创建" });
  await expect(created.getByRole("link")).toHaveCount(10);
  await expect(page.getByRole("navigation", { name: "首页导航" })).toBeHidden();
  const first = created.getByRole("link").first();
  await expect(first).toHaveCSS("grid-template-columns", /^\S+ \S+$/);
  expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBe(
    390,
  );

  await first.focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/doc\/.+\?mode=dark$/);
  await expect(page.locator(".markdown-body")).toBeVisible();
});
