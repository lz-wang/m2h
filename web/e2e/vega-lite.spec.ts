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
        placeholder: container.querySelector(".m2h-rich-visual-error") !== null,
        source: container.textContent?.slice(0, 20) ?? "",
      }),
    ),
  );
  // The two broken charts collapse into the shared placeholder (never their
  // JSON source), with no SVG, no marker, and a hidden trigger; the valid
  // chart after them renders normally.
  expect(states[0]?.hasSVG).toBe(false);
  expect(states[0]?.lightbox).toBe("off");
  expect(states[0]?.triggerHidden).toBe(true);
  expect(states[0]?.placeholder).toBe(true);
  expect(states[0]?.source).not.toContain("{");
  expect(states[1]?.hasSVG).toBe(false);
  expect(states[1]?.lightbox).toBe("off");
  expect(states[1]?.placeholder).toBe(true);
  expect(states[2]?.hasSVG).toBe(true);
  expect(states[2]?.lightbox).toBe("on");
  expect(states[2]?.triggerHidden).toBe(false);
});

test("isolates unsupported external resources without leaving the origin", async ({
  page,
}) => {
  const warnings: string[] = [];
  page.on("console", (message) => {
    const text = message.text();
    if (text.includes("Failed to render Vega-Lite chart")) {
      warnings.push(text);
    }
  });

  const tracker = await openUntilChartsSettle(page, securityPath);

  // No request ever leaves for the specs' remote resources — neither the
  // data URLs (rejected up front by the self-contained preflight) nor the
  // image mark's href (rejected by the host loader before any fetch), so
  // the contract holds even where no CSP exists (exported HTML).
  const external = tracker
    .urls()
    .filter((url) => !url.startsWith("http://127.0.0.1:"));
  expect(external).toEqual([]);

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
        source: container.textContent ?? "",
        marks: container.querySelectorAll("g[class~='role-mark'] > *").length,
      }),
    ),
  );
  expect(states).toHaveLength(5);
  // The external-data charts (top-level and inside a layer) fail the
  // self-contained preflight before any embed: placeholder in, no SVG, no
  // Lightbox, and their source text (with the remote URLs) stays off the
  // page — the ordinary isolated-chart contract, not the empty-frame
  // "success" Vega produces when only the loader denies the fetch.
  expect(states[0]?.hasSVG).toBe(false);
  expect(states[0]?.lightbox).toBe("off");
  expect(states[0]?.triggerHidden).toBe(true);
  expect(states[0]?.source).not.toContain("example.invalid/data.csv");
  expect(states[1]?.hasSVG).toBe(false);
  expect(states[1]?.lightbox).toBe("off");
  expect(states[1]?.source).not.toContain("example.invalid/nested.json");
  expect(
    warnings.filter((text) => text.includes("self-contained")),
  ).toHaveLength(2);

  // The image-mark chart renders its frame — the external image itself never
  // loads, but the chart is not an isolated failure.
  expect(states[2]?.hasSVG).toBe(true);
  // The hyperlink chart and the plain self-contained chart both drew marks.
  expect(states[3]?.hasSVG).toBe(true);
  expect(states[3]?.marks).toBeGreaterThan(0);
  expect(states[4]?.hasSVG).toBe(true);
  expect(states[4]?.marks).toBeGreaterThan(0);

  // The chart-generated hyperlink follows the reader-wide link policy: the
  // cross-origin click opens a new tab — not this tab — with noopener. Vega
  // synthesizes the anchor on click and routes it through the host loader's
  // href sanitization, which is where the target/rel policy lands. The
  // context route fakes the unreachable origin so the popup commits and its
  // URL can be read.
  await page
    .context()
    .route("https://example.invalid/**", (route) =>
      route.fulfill({ status: 200, contentType: "text/html", body: "linked" }),
    );
  const popupPromise = page.waitForEvent("popup");
  await page
    .locator(".m2h-vega-lite")
    .nth(3)
    .locator("g[class~='role-mark'] > *")
    .first()
    .click();
  const popup = await popupPromise;
  await popup.waitForLoadState("domcontentloaded");
  expect(popup.url()).toBe("https://example.invalid/linked");
  // noopener: the new tab holds no window.opener reference back to the
  // document (Playwright's popup.opener() tracks the CDP opener instead, so
  // the probe has to run inside the popup itself).
  expect(await popup.evaluate(() => window.opener)).toBeNull();
  await popup.close();
  // The reader tab stayed on the document the whole time.
  expect(page.url()).toContain(securityPath);
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

test("repaints charts when the theme flips while the runtime is loading", async ({
  page,
}) => {
  // The race the generation guard alone could not close: a theme toggle that
  // lands while the initial enhancement is still awaiting the runtime
  // download. The chart containers do not exist yet, so the naive repaint
  // found no targets — and a repaint that invalidated the initial render
  // instead left the raw fenced blocks behind for good. Delaying the runtime
  // trio keeps that window open long enough to toggle inside it.
  await page.route("**/runtime/vega*.js", async (route) => {
    await new Promise((resolve) => {
      setTimeout(resolve, 800);
    });
    await route.continue();
  });
  const tracker = trackRequests(page);
  await openDocument(page, themePath);

  // Prove the toggle below happens inside the loading window: the fenced
  // block is still raw and no embed runtime has attached yet.
  await page.waitForFunction(() => {
    const raw = document.querySelector(
      "pre > code.language-vega-lite, pre > code.language-vegalite",
    );
    return raw !== null && window.vegaEmbed === undefined;
  });

  await page.getByRole("button", { name: /^显示主题：/ }).click();
  await pickTheme(page, "深色", true);

  // The initial render survived the toggle: the blocks convert and paint,
  // then the queued dark repaint re-embeds in the dark palette.
  await tracker.waitForIdleCharts();
  await expect(
    page.locator("pre > code.language-vega-lite, pre > code.language-vegalite"),
  ).toHaveCount(0);
  await expect(page.locator(".m2h-vega-lite")).toHaveCount(1);
  const darkMarks = await page.evaluate(
    () =>
      document
        .querySelector(".m2h-vega-lite")
        ?.querySelectorAll("g[class~='role-mark'] > *").length ?? 0,
  );
  expect(darkMarks).toBeGreaterThan(0);
  // The Mermaid diagram in the same document repainted alongside the chart.
  await expect(
    page.locator(".m2h-mermaid-frame > .mermaid > svg"),
  ).toBeVisible();

  // Toggling back still repaints (the queue was not wedged by the wait) and
  // the chart chrome follows the palette again.
  const darkLabel = await page.evaluate(() => {
    const label = document.querySelector(
      ".m2h-vega-lite svg text",
    ) as SVGTextElement | null;
    return label?.getAttribute("fill") ?? "";
  });
  await pickTheme(page, "浅色", false);
  const lightLabel = await page.evaluate(() => {
    const label = document.querySelector(
      ".m2h-vega-lite svg text",
    ) as SVGTextElement | null;
    return label?.getAttribute("fill") ?? "";
  });
  expect(lightLabel).not.toBe(darkLabel);
});

test("ends in the latest palette after a rapid light → dark → light toggle", async ({
  page,
}) => {
  await openUntilChartsSettle(page, themePath);

  const frame = page.locator(".m2h-vega-lite-frame").first();
  const container = page.locator(".m2h-vega-lite").first();
  const trigger = frame.locator(":scope > .m2h-lightbox-trigger");
  const frameElement = await frame.elementHandle();
  const containerElement = await container.elementHandle();
  const triggerElement = await trigger.elementHandle();
  const lightLabel = await page.evaluate(() => {
    const label = document.querySelector(
      ".m2h-vega-lite svg text",
    ) as SVGTextElement | null;
    return label?.getAttribute("fill") ?? "";
  });

  // Two picks back to back, no wait between them: the middle dark repaint
  // must be skipped or run to completion — never overlapped with the light
  // one on the same container, where interleaved embed writes would corrupt
  // the chart DOM.
  await page.getByRole("button", { name: /^显示主题：/ }).click();
  await page.getByRole("menuitemradio", { name: "深色" }).click();
  await page.getByRole("menuitemradio", { name: "浅色" }).click();

  await page.waitForFunction(
    () => !document.documentElement.classList.contains("m2h-mode-dark"),
  );
  await waitForBodyQuiet(page);

  // One chart, still converted, in the final light palette.
  await expect(
    page.locator("pre > code.language-vega-lite, pre > code.language-vegalite"),
  ).toHaveCount(0);
  await expect(page.locator(".m2h-vega-lite")).toHaveCount(1);
  await expect(page.locator(".m2h-vega-lite-frame")).toHaveCount(1);
  const marks = await page.evaluate(
    () =>
      document
        .querySelector(".m2h-vega-lite")
        ?.querySelectorAll("g[class~='role-mark'] > *").length ?? 0,
  );
  expect(marks).toBeGreaterThan(0);
  const finalLabel = await page.evaluate(() => {
    const label = document.querySelector(
      ".m2h-vega-lite svg text",
    ) as SVGTextElement | null;
    return label?.getAttribute("fill") ?? "";
  });
  expect(finalLabel).toBe(lightLabel);

  // DOM identity survived the rapid round trip and nothing stacked.
  expect(await frameElement?.evaluate((el) => el.isConnected)).toBe(true);
  expect(await containerElement?.evaluate((el) => el.isConnected)).toBe(true);
  expect(await triggerElement?.evaluate((el) => el.isConnected)).toBe(true);
  await expect(page.locator(".vega-actions")).toHaveCount(0);
  await expect(
    page.locator("#vg-tooltip-element .vg-tooltip-element"),
  ).toHaveCount(0);
});

test("browses charts, diagrams, and images in one lightbox sequence", async ({
  page,
}) => {
  await openUntilChartsSettle(page, lightboxPath);

  // A rendered Vega-Lite SVG can carry an href-bearing mark. The Lightbox
  // snapshots that real SVG: the body keeps the link navigable while the
  // snapshot keeps it hittable and selectable, with only its navigation
  // blocked (the component intercepts link clicks in the capture phase).
  const injectChartLink = () =>
    page.locator(".markdown-body .m2h-vega-lite svg").evaluate((svg) => {
      const namespace = "http://www.w3.org/2000/svg";
      const link = document.createElementNS(namespace, "a");
      link.setAttribute("href", "#vega-lite-chart-link");
      const target = document.createElementNS(namespace, "rect");
      const viewBox = svg.viewBox.baseVal;
      target.setAttribute("x", String(viewBox.x));
      target.setAttribute("y", String(viewBox.y));
      target.setAttribute("width", String(viewBox.width));
      target.setAttribute("height", String(viewBox.height));
      target.setAttribute("fill", "transparent");
      link.append(target);
      svg.append(link);
    });
  await injectChartLink();
  const bodyChartLink = page.locator(
    ".markdown-body .m2h-vega-lite a[href='#vega-lite-chart-link']",
  );
  await expect(bodyChartLink).toHaveCount(1);
  await bodyChartLink.first().click();
  await expect(page).toHaveURL(/#vega-lite-chart-link$/);
  await page.evaluate(() => {
    history.replaceState(null, "", location.pathname);
  });
  // Following the in-document anchor routes the reader through a full
  // document reload (/api/document), so the chart SVG — and with it the
  // injected link — is rebuilt from scratch. Re-inject and pin the count
  // before opening the Lightbox, whose snapshot then carries exactly one.
  await waitForBodyQuiet(page);
  await expect(bodyChartLink).toHaveCount(0);
  await injectChartLink();
  await expect(bodyChartLink).toHaveCount(1);

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

  // The chart remains a native inline SVG on the light theme's diagram canvas.
  const stageState = await page.evaluate(() => {
    const stage = document.querySelector<HTMLElement>(".image-lightbox-stage");
    const svg = document.querySelector(".image-lightbox-vector > svg");
    return {
      kind: stage?.dataset.visualKind,
      background: stage === null ? "" : getComputedStyle(stage).backgroundColor,
      svgCount: svg === null ? 0 : 1,
    };
  });
  expect(stageState.kind).toBe("vega-lite");
  expect(stageState.background).toBe("rgb(255, 255, 255)");
  expect(stageState.svgCount).toBe(1);
  await expect(page.locator(".image-lightbox-image")).toHaveCount(0);

  const lightboxChartLink = page.locator(
    ".image-lightbox-vector a[href='#vega-lite-chart-link']",
  );
  await expect(lightboxChartLink).toHaveCount(1);
  const linkBox = await lightboxChartLink.first().boundingBox();
  if (linkBox === null) {
    throw new Error("lightbox chart link was not rendered");
  }
  await page.mouse.click(
    linkBox.x + linkBox.width / 2,
    linkBox.y + linkBox.height / 2,
  );
  await expect(page).not.toHaveURL(/#vega-lite-chart-link$/);

  // The zoom contract lives on the real root <svg>, not the wrapper: a
  // wrapper that grows around a diagram pinned by its own inline max-width
  // must fail here. With pan and rotation at rest the growth also stays
  // centered instead of drifting sideways. (The settle poll rides out the
  // stage's enter transition, during which Chromium can transiently report
  // empty content quads.)
  const svg = page.locator(".image-lightbox-vector > svg");
  await expect
    .poll(async () => (await svg.boundingBox())?.width ?? 0, {
      timeout: 5_000,
    })
    .toBeGreaterThan(0);
  const before = await svg.boundingBox();
  if (before === null) {
    throw new Error("lightbox svg was not rendered");
  }
  await page.getByRole("button", { name: "放大图片" }).click();
  const after = await svg.boundingBox();
  if (after === null) {
    throw new Error("lightbox svg was not rendered");
  }
  expect(after.width / before.width).toBeCloseTo(1.25, 1);
  expect(after.height / before.height).toBeCloseTo(1.25, 1);
  expect(
    Math.abs(after.x + after.width / 2 - (before.x + before.width / 2)),
  ).toBeLessThanOrEqual(1);
  expect(
    Math.abs(after.y + after.height / 2 - (before.y + before.height / 2)),
  ).toBeLessThanOrEqual(1);
  await expect(
    page.locator(".image-lightbox-vector-transform"),
  ).not.toHaveAttribute("style", /scale\(/);

  await page.getByRole("button", { name: "上一项" }).click();
  await expect(counter).toHaveText("2 / 3");
  await page.getByRole("button", { name: "上一项" }).click();
  await expect(counter).toHaveText("1 / 3");
  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(counter).toHaveCount(0);
});
