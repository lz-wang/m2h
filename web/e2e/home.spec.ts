import { expect, test } from "@playwright/test";

test("directory home opens document links and survives history navigation and reload", async ({
  page,
}) => {
  const requests: string[] = [];
  page.on("request", (request) => {
    requests.push(new URL(request.url()).pathname);
  });

  await page.goto("/");
  const home = page.getByRole("article", { name: "文档首页" });
  const created = page.getByRole("region", { name: "最新创建" });
  const updated = page.getByRole("region", { name: "最近更新" });
  await expect(home).toBeVisible();
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
  await expect(home).toBeVisible();
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

test("sidebar home stays clickable while the file tree scrolls", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1280, height: 480 });
  await page.goto("/doc/README.md");
  await expect(page.locator(".markdown-body")).toBeVisible();

  const sidebarHeader = page.locator('[data-slot="sidebar-header"]');
  const home = sidebarHeader.getByRole("button", { name: "文档首页" });
  const treeViewport = page.locator(
    ".tree-scroll [data-slot='scroll-area-viewport']",
  );
  await expect(home).toBeVisible();
  await expect(
    page.locator(".reader-toolbar").getByRole("button", { name: "文档首页" }),
  ).toHaveCount(0);
  const initial = await home.boundingBox();
  await treeViewport.hover();
  await page.mouse.wheel(0, 640);
  await expect
    .poll(() => treeViewport.evaluate((viewport) => viewport.scrollTop))
    .toBeGreaterThan(0);
  expect(await home.boundingBox()).toEqual(initial);
  await home.click();
  await expect(page).toHaveURL(/\/$/);
  await expect(home).toHaveAttribute("aria-current", "page");
});

test("keeps outline controls on document pages and preserves their preference", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/doc/toc-scroll.md");
  const outline = page.getByRole("navigation", { name: "文档目录" });
  await expect(outline).toBeVisible();
  await page.getByRole("button", { name: "隐藏文档目录" }).click();

  await page.getByRole("button", { name: "文档首页" }).click();
  await expect(page).toHaveURL(/\/\?toc=false$/);
  const home = page.getByRole("article", { name: "文档首页" });
  await expect(home.getByRole("region", { name: "最新创建" })).toBeVisible();
  await expect(home.getByRole("heading", { name: "文档首页" })).toHaveCount(0);
  await expect(page.getByRole("navigation", { name: "首页导航" })).toHaveCount(
    0,
  );
  await expect(page.locator(".reader-toc-toggle")).toHaveCount(0);

  await page.goBack();
  await expect(
    page.getByRole("button", { name: "显示文档目录" }),
  ).toBeVisible();
  await expect(outline).toBeHidden();
  await page.getByRole("button", { name: "显示文档目录" }).click();
  await expect(outline).toBeVisible();
});

test("aligns overview icons, labels and values on desktop", async ({
  page,
}) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto("/?mode=dark");
  await expect(page.getByRole("navigation", { name: "首页导航" })).toHaveCount(
    0,
  );
  await expect(
    page.getByRole("searchbox", { name: "按文件名或路径筛选文件" }),
  ).toBeVisible();
  const overview = page.locator(".document-home-layout > :first-child");
  await expect(overview).toHaveClass("document-home-overview");
  await expect(overview.locator("dd time")).toBeVisible();

  const offsets = await overview
    .locator(":scope > div")
    .evaluateAll((metrics) =>
      metrics.map((metric) => {
        const centers = Array.from(
          metric.querySelectorAll("dt, dd, dt svg"),
          (element) => {
            const { y, height } = element.getBoundingClientRect();
            return y + height / 2;
          },
        );
        return Math.max(...centers) - Math.min(...centers);
      }),
    );
  expect(offsets).toHaveLength(3);
  for (const offset of offsets) {
    expect(offset).toBeLessThanOrEqual(0.5);
  }
});

test("home lists remain readable on a narrow dark viewport", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/?mode=dark");
  await expect(page.locator("html")).toHaveClass(/dark/);
  const created = page.getByRole("region", { name: "最新创建" });
  await expect(created.getByRole("link")).toHaveCount(10);
  const labels = page.locator(".document-home-overview dt");
  const values = page.locator(".document-home-overview dd");
  for (let index = 0; index < 3; index += 1) {
    const label = await labels.nth(index).boundingBox();
    const value = await values.nth(index).boundingBox();
    if (!label || !value) throw new Error("missing overview metric");
    expect(value.y).toBeGreaterThanOrEqual(label.y + label.height);
    expect(value.x).toBeCloseTo(label.x, 1);
  }
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
