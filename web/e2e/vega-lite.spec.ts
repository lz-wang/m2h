import { expect, type Page, test } from "@playwright/test";

// Real-browser regressions for Vega-Lite statistical charts. The unit tests
// mock vegaEmbed; only this spec executes the real vega → vega-lite →
// vega-embed chain against the /runtime/ assets the Go binary embeds, which
// is where the pinned host policy (mode/renderer/actions, the deny-network
// loader, and usermeta stripping) and the Vega-Embed internals actually meet.

const basicPath = "/doc/vega-lite-basic.md";
const themePath = "/doc/vega-lite-theme.md";
const invalidPath = "/doc/vega-lite-invalid.md";
const lightboxPath = "/doc/vega-lite-lightbox.md";
const securityPath = "/doc/vega-lite-security.md";

const VEGA_RUNTIME_SCRIPTS = [
  "/runtime/vega.min.js",
  "/runtime/vega-lite.min.js",
  "/runtime/vega-embed.min.js",
];

// Wait until no mutation has touched the Markdown body for 250ms — the
// generic "async enhancement settled" signal. Chart rendering leaves no
// marker that distinguishes a failed chart from one not yet rendered, and a
// theme repaint replaces the SVG asynchronously, so DOM quiet is the only
// signal that covers every path.
function waitForBodyQuiet(page: Page): Promise<void> {
  return page.waitForFunction(
    () =>
      new Promise<boolean>((resolve) => {
        const body = document.querySelector(".markdown-body") ?? document.body;
        const observer = new MutationObserver(() => {
          window.clearTimeout(timer);
          timer = window.setTimeout(finish, 250);
        });
        let timer = window.setTimeout(finish, 250);
        function finish() {
          observer.disconnect();
          resolve(true);
        }
        observer.observe(body, {
          childList: true,
          subtree: true,
          characterData: true,
        });
      }),
  );
}

// Track every request the page makes, so the lazy-loading and security
// assertions can observe exactly what the document pulled.
function trackRequests(page: Page): {
  urls: () => string[];
  waitForIdleCharts: () => Promise<void>;
} {
  const urls: string[] = [];
  page.on("request", (request) => {
    urls.push(request.url());
  });
  return {
    urls: () => urls,
    // Wait until the fenced blocks are all consumed and the body went
    // quiet (charts render asynchronously after the runtimes load).
    waitForIdleCharts: async () => {
      await page.waitForFunction(
        () =>
          document.querySelectorAll(
            "pre > code.language-vega-lite, pre > code.language-vegalite",
          ).length === 0,
      );
      await waitForBodyQuiet(page);
    },
  };
}

async function openDocument(page: Page, path: string) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(path);
  await page.waitForFunction(
    () => document.querySelector(".markdown-body") !== null,
  );
}

async function openUntilChartsSettle(page: Page, path: string) {
  const tracker = trackRequests(page);
  await openDocument(page, path);
  await tracker.waitForIdleCharts();
  return tracker;
}

// The theme menu is Base UI's non-modal menu, which stays open after a
// pick — so the round trip opens it once and clicks both items in sequence.
// Each pick also kicks an asynchronous chart repaint; let the body settle
// before reading anything.
async function pickTheme(
  page: Page,
  item: string,
  dark: boolean,
): Promise<void> {
  await page.getByRole("menuitemradio", { name: item }).click();
  await page.waitForFunction(
    (isDark) =>
      document.documentElement.classList.contains("m2h-mode-dark") === isDark,
    dark,
  );
  await waitForBodyQuiet(page);
}

test("renders every chart shape through the real runtime trio", async ({
  page,
}) => {
  await openUntilChartsSettle(page, basicPath);

  // Six fenced blocks (bar, line, scatter, layered, tooltip, alias) all
  // render: no source block survives, every container holds a real SVG with
  // rendered marks, and none of them leaks the spec source into the body.
  await expect(
    page.locator("pre > code.language-vega-lite, pre > code.language-vegalite"),
  ).toHaveCount(0);
  const containers = page.locator(".m2h-vega-lite");
  await expect(containers).toHaveCount(6);
  const markCounts = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".m2h-vega-lite > svg")).map(
      (svg) => svg.querySelectorAll("path, rect, circle").length,
    ),
  );
  expect(markCounts).toHaveLength(6);
  for (const count of markCounts) {
    expect(count).toBeGreaterThan(0);
  }
  await expect(page.getByText('"$schema"')).toHaveCount(0);

  // Vega-Embed's own actions menu (Export / Source / Editor) must stay off.
  await expect(page.locator(".vega-embed .vega-actions")).toHaveCount(0);
  // Charts are SVG, not canvas.
  await expect(page.locator(".m2h-vega-lite canvas")).toHaveCount(0);
});

test("loads the Vega runtime trio only for documents that use it", async ({
  page,
}) => {
  // A plain document and a Mermaid-only document must not request any Vega
  // asset; the chart document pulls all three exactly once each.
  const plain = trackRequests(page);
  await openDocument(page, "/doc/scroll.md");
  await page.waitForFunction(
    () => document.querySelector(".markdown-body p") !== null,
  );
  expect(plain.urls().filter((url) => url.includes("/runtime/vega"))).toEqual(
    [],
  );

  const mermaidOnly = trackRequests(page);
  await openDocument(page, "/doc/mermaid-zenuml.md");
  await page.waitForFunction(
    () => document.querySelector(".m2h-mermaid-frame svg") !== null,
  );
  expect(
    mermaidOnly.urls().filter((url) => url.includes("/runtime/vega")),
  ).toEqual([]);

  const charts = await openUntilChartsSettle(page, basicPath);
  for (const script of VEGA_RUNTIME_SCRIPTS) {
    const hits = charts
      .urls()
      .filter((url) => url.endsWith(script) || url.includes(`${script}?`));
    expect(hits, script).toHaveLength(1);
  }
});

test("isolates invalid charts and keeps valid ones rendering", async ({
  page,
}) => {
  await openUntilChartsSettle(page, invalidPath);

  const containers = page.locator(".m2h-vega-lite");
  await expect(containers).toHaveCount(3);
  const states = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".m2h-vega-lite")).map(
      (container) => ({
        hasSVG: container.querySelector("svg") !== null,
        lightbox: container.dataset.m2hLightboxItem === "true" ? "on" : "off",
        triggerHidden:
          container
            .closest(".m2h-vega-lite-frame")
            ?.querySelector<HTMLButtonElement>(":scope > .m2h-lightbox-trigger")
            ?.hidden ?? true,
        source: container.textContent?.slice(0, 20) ?? "",
      }),
    ),
  );
  // The two broken charts keep their source text with no SVG, no marker, and
  // a hidden trigger; the valid chart after them renders normally.
  expect(states[0]?.hasSVG).toBe(false);
  expect(states[0]?.lightbox).toBe("off");
  expect(states[0]?.triggerHidden).toBe(true);
  expect(states[1]?.hasSVG).toBe(false);
  expect(states[1]?.lightbox).toBe("off");
  expect(states[2]?.hasSVG).toBe(true);
  expect(states[2]?.lightbox).toBe("on");
  expect(states[2]?.triggerHidden).toBe(false);
});

test("denies external data URLs at the loader, not via the CSP", async ({
  page,
}) => {
  const denials: string[] = [];
  page.on("console", (message) => {
    const text = message.text();
    if (text.includes("external Vega-Lite data loading")) {
      denials.push(text);
    }
  });

  const tracker = await openUntilChartsSettle(page, securityPath);

  // No request ever leaves for the spec's remote data source — the host
  // loader rejects before any fetch, so the contract holds even where no CSP
  // exists (exported HTML). The denial itself must be the loader's, not a
  // CSP block happening to intercept a fetch Vega already issued.
  const external = tracker
    .urls()
    .filter((url) => !url.startsWith("http://127.0.0.1:"));
  expect(external).toEqual([]);
  expect(denials.length).toBeGreaterThan(0);

  // Vega renders the denied chart as its empty frame — axis chrome with no
  // data marks — while the self-contained chart after it drew its bars
  // (Vega groups data marks under g.role-mark).
  const dataMarks = await page.evaluate(() =>
    Array.from(document.querySelectorAll(".m2h-vega-lite")).map(
      (container) =>
        container.querySelectorAll("g[class~='role-mark'] > *").length,
    ),
  );
  expect(dataMarks).toHaveLength(2);
  expect(dataMarks[0]).toBe(0);
  expect(dataMarks[1]).toBeGreaterThan(0);
});

test("repaints charts across a light → dark → light round trip", async ({
  page,
}) => {
  await openUntilChartsSettle(page, themePath);

  const frame = page.locator(".m2h-vega-lite-frame").first();
  const container = page.locator(".m2h-vega-lite").first();
  const trigger = frame.locator(":scope > .m2h-lightbox-trigger");
  const frameElement = await frame.elementHandle();
  const containerElement = await container.elementHandle();
  const triggerElement = await trigger.elementHandle();

  // Axis label color reflects the reader theme: dark mode flips the chrome
  // color while the author's mark palette stays put.
  const lightLabel = await page.evaluate(() => {
    const label = document.querySelector(
      ".m2h-vega-lite svg text",
    ) as SVGTextElement | null;
    return label?.getAttribute("fill") ?? "";
  });

  await page.getByRole("button", { name: /^显示主题：/ }).click();
  await pickTheme(page, "深色", true);
  await page.waitForFunction(
    () => document.querySelector(".m2h-vega-lite > svg") !== null,
  );
  const darkLabel = await page.evaluate(() => {
    const label = document.querySelector(
      ".m2h-vega-lite svg text",
    ) as SVGTextElement | null;
    return label?.getAttribute("fill") ?? "";
  });
  expect(darkLabel).not.toBe(lightLabel);

  await pickTheme(page, "浅色", false);
  await page.waitForFunction(
    () => document.querySelector(".m2h-vega-lite > svg") !== null,
  );

  // DOM identity survived both repaints: same frame, container, trigger.
  expect(await frameElement?.evaluate((el) => el.isConnected)).toBe(true);
  expect(await containerElement?.evaluate((el) => el.isConnected)).toBe(true);
  expect(await triggerElement?.evaluate((el) => el.isConnected)).toBe(true);

  // One chart, one embed container: repaints replaced content in place
  // instead of stacking Vega-Embed wrappers, tooltips, or action menus.
  await expect(page.locator(".m2h-vega-lite")).toHaveCount(1);
  await expect(page.locator(".m2h-vega-lite-frame")).toHaveCount(1);
  await expect(page.locator(".vega-actions")).toHaveCount(0);
  await expect(
    page.locator("#vg-tooltip-element .vg-tooltip-element"),
  ).toHaveCount(0);

  // The Mermaid diagram in the same document repainted alongside the chart.
  await expect(
    page.locator(".m2h-mermaid-frame > .mermaid > svg"),
  ).toBeVisible();
});

test("browses charts, diagrams, and images in one lightbox sequence", async ({
  page,
}) => {
  await openUntilChartsSettle(page, lightboxPath);

  // Three visual items in document order: image → Mermaid → Vega-Lite.
  const markers = await page.evaluate(() =>
    Array.from(
      document.querySelectorAll('[data-m2h-lightbox-item="true"]'),
    ).map((element) =>
      element instanceof HTMLImageElement
        ? "image"
        : element.classList.contains("m2h-vega-lite")
          ? "vega-lite"
          : "mermaid",
    ),
  );
  expect(markers).toEqual(["image", "mermaid", "vega-lite"]);

  // Open through the chart's own trigger and walk the whole sequence. The
  // trigger is hover-gated until its frame is hovered (same as images).
  await page.locator(".m2h-vega-lite-frame").hover();
  await page.locator(".m2h-vega-lite-frame > .m2h-lightbox-trigger").click();
  const counter = page.locator(
    '.image-lightbox-counter > span[aria-hidden="true"]',
  );
  await expect(counter).toHaveText("3 / 3");
  await page.getByRole("button", { name: "上一项" }).click();
  await expect(counter).toHaveText("2 / 3");
  await page.getByRole("button", { name: "上一项" }).click();
  await expect(counter).toHaveText("1 / 3");
  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(counter).toHaveCount(0);
});
