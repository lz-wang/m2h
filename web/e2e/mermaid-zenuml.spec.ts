import { expect, type Page, test } from "@playwright/test";

// Real-browser regression for the ZenUML external diagram. Mermaid Core does
// not know the `zenuml` diagram type: without the vendored plugin registered
// through registerExternalDiagrams, a perfectly valid diagram degrades into
// Mermaid's "Syntax error in text" fallback. The unit tests mock the plugin
// module — only this spec executes the real load → register → initialize →
// render chain against the /runtime/mermaid-zenuml/ assets the Go binary
// embeds.

const documentPath = "/doc/mermaid-zenuml.md";
const stressDocumentPath = "/doc/mermaid-zenuml-stress.md";
const ordinaryDocumentPath = "/doc/scroll.md";

interface HostStyleSnapshot {
  backgroundToken: string;
  bodyBackground: string;
  toolbarBackground: string;
  tocBackground: string;
  markdownBackground: string;
  markdownColor: string;
  markdownBoxSizing: string;
  headStylesheetCount: number;
  oversizedHeadStyleCount: number;
}

async function readHostStyles(page: Page): Promise<HostStyleSnapshot> {
  return page.evaluate(() => {
    const toolbar = document.querySelector(".reader-toolbar");
    const toc = document.querySelector(".reader-toc");
    const markdown = document.querySelector(".markdown-body");
    if (!(toolbar instanceof HTMLElement)) {
      throw new Error("reader toolbar was not rendered");
    }
    if (!(toc instanceof HTMLElement)) {
      throw new Error("reader TOC was not rendered");
    }
    if (!(markdown instanceof HTMLElement)) {
      throw new Error("Markdown body was not rendered");
    }
    const toolbarStyle = getComputedStyle(toolbar);
    const tocStyle = getComputedStyle(toc);
    const markdownStyle = getComputedStyle(markdown);
    const headStylesheets = Array.from(document.head.children).filter(
      (element) =>
        element instanceof HTMLStyleElement ||
        (element instanceof HTMLLinkElement && element.rel === "stylesheet"),
    );
    return {
      backgroundToken: getComputedStyle(
        document.documentElement,
      ).getPropertyValue("--background"),
      bodyBackground: getComputedStyle(document.body).backgroundColor,
      toolbarBackground: toolbarStyle.backgroundColor,
      tocBackground: tocStyle.backgroundColor,
      markdownBackground: markdownStyle.backgroundColor,
      markdownColor: markdownStyle.color,
      markdownBoxSizing: markdownStyle.boxSizing,
      headStylesheetCount: headStylesheets.length,
      oversizedHeadStyleCount: headStylesheets.filter(
        (element) => (element.textContent?.length ?? 0) > 100_000,
      ).length,
    };
  });
}

async function openDocument(page: Page, path: string) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(path);
  await page.waitForFunction(
    () => document.querySelector(".markdown-body") !== null,
  );
}

async function openZenUMLDocument(page: Page) {
  await page.setViewportSize({ width: 1280, height: 900 });
  await page.goto(documentPath);
  // The diagram must finish rendering — its SVG lands inside the frame —
  // before any dependent assertion runs.
  await page.waitForFunction(
    () => document.querySelector(".m2h-mermaid-frame svg") !== null,
  );
}

async function expectRenderedDiagram(page: Page) {
  // ZenUML nests its own <svg> arrowheads inside the diagram, so the root
  // diagram SVG is matched as the container's direct child, not by descendant.
  const svg = page.locator(".m2h-mermaid-frame .mermaid > svg");
  await expect(svg).toBeVisible();
  // The source block is consumed by the swap: what remains is the diagram,
  // never the raw zenuml source text.
  await expect(page.locator("pre > code.language-mermaid")).toHaveCount(0);
  // The classic symptom of a ZenUML plugin that never registered.
  await expect(page.getByText("Syntax error in text")).toHaveCount(0);
  // A real sequence diagram, not an empty SVG shell.
  const children = await page.evaluate(
    () =>
      document.querySelector(".m2h-mermaid-frame .mermaid > svg")
        ?.childElementCount ?? 0,
  );
  expect(children).toBeGreaterThan(0);
  return svg;
}

async function switchToDarkTheme(page: Page) {
  await page.getByRole("button", { name: /^显示主题：/ }).click();
  await page.getByRole("menuitemradio", { name: "深色" }).click();
  await page.waitForFunction(() =>
    document.documentElement.classList.contains("m2h-mode-dark"),
  );
}

test("renders the ZenUML sequence diagram via the registered plugin", async ({
  page,
}) => {
  await openZenUMLDocument(page);

  const svg = await expectRenderedDiagram(page);
  // The official demo diagram names its participants; a rendered sequence
  // diagram carries them as SVG text.
  await expect(svg).toContainText("Alice");
  await expect(svg).toContainText("John");
});

test("opens the ZenUML diagram in the shared lightbox", async ({ page }) => {
  await openZenUMLDocument(page);
  await expectRenderedDiagram(page);

  // ZenUML is an SVG like any other diagram: the shared trigger and popup
  // must work without any ZenUML-specific handling.
  await page.locator(".m2h-mermaid-frame").hover();
  await page.locator(".m2h-mermaid-frame .m2h-lightbox-trigger").click();

  const popup = page.locator(".image-lightbox");
  await expect(popup).toBeVisible();
  await expect(page.locator(".image-lightbox-image")).toHaveAttribute(
    "src",
    /^data:image\/svg\+xml/,
  );

  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
  await expect(popup).toBeHidden();
});

test("repaints the ZenUML diagram when the theme switches", async ({
  page,
}) => {
  await openZenUMLDocument(page);
  await expectRenderedDiagram(page);

  const lightPalette = await page.evaluate(() => {
    const svg = document.querySelector(".m2h-mermaid-frame .mermaid > svg");
    const participant = svg?.querySelector(".participant-box");
    const label = svg?.querySelector(".participant-label");
    return {
      theme: svg?.getAttribute("data-m2h-zenuml-theme"),
      participantFill: participant ? getComputedStyle(participant).fill : null,
      labelFill: label ? getComputedStyle(label).fill : null,
    };
  });
  expect(lightPalette).toEqual({
    theme: "light",
    participantFill: "rgb(255, 255, 255)",
    labelFill: "rgb(34, 34, 34)",
  });

  // Mark the light-painted SVG so the dark repaint must produce a brand-new
  // element, not keep the stale one.
  await page.evaluate(() => {
    document
      .querySelector(".m2h-mermaid-frame .mermaid > svg")
      ?.setAttribute("data-m2h-stale-paint", "1");
  });

  await switchToDarkTheme(page);

  await page.waitForFunction(
    () =>
      document.querySelector(
        ".m2h-mermaid-frame .mermaid > svg[data-m2h-stale-paint]",
      ) === null,
  );
  await expectRenderedDiagram(page);
  const darkPalette = await page.evaluate(() => {
    const svg = document.querySelector(".m2h-mermaid-frame .mermaid > svg");
    const participant = svg?.querySelector(".participant-box");
    const label = svg?.querySelector(".participant-label");
    const message = svg?.querySelector(".message-line");
    return {
      theme: svg?.getAttribute("data-m2h-zenuml-theme"),
      participantFill: participant ? getComputedStyle(participant).fill : null,
      labelFill: label ? getComputedStyle(label).fill : null,
      messageStroke: message ? getComputedStyle(message).stroke : null,
      scopedThemeStyles: svg?.querySelectorAll(
        '[data-m2h-zenuml-theme-style="dark"]',
      ).length,
    };
  });
  expect(darkPalette).toEqual({
    theme: "dark",
    participantFill: "rgb(31, 32, 32)",
    labelFill: "rgb(204, 204, 204)",
    messageStroke: "rgb(204, 204, 204)",
    scopedThemeStyles: 1,
  });

  // The theme lives inside the SVG, so the shared Lightbox snapshot must keep
  // the same dark palette instead of reverting to the upstream light artwork.
  await page.locator(".m2h-mermaid-frame").first().hover();
  await page
    .locator(".m2h-mermaid-frame .m2h-lightbox-trigger")
    .first()
    .click();
  const snapshotSource = await page
    .locator(".image-lightbox-image")
    .getAttribute("src");
  expect(snapshotSource).not.toBeNull();
  // Decode the data URL with decodeURIComponent instead of fetching it: the
  // page ships a strict connect-src 'self' CSP, and fetching data: URLs from
  // the page context would be refused by design.
  const snapshotSVG = await page.evaluate((source) => {
    if (source === null) {
      return "";
    }
    return decodeURIComponent(source.slice(source.indexOf(",") + 1));
  }, snapshotSource);
  expect(snapshotSVG).toContain('data-m2h-zenuml-theme="dark"');
  expect(snapshotSVG).toContain('data-m2h-zenuml-theme-style="dark"');
  expect(snapshotSVG).toContain("fill: #1f2020");
  await page.getByRole("button", { name: "关闭视觉内容预览" }).click();
});

for (const mode of ["light", "dark"] as const) {
  test(`keeps ${mode} host styles unchanged after ZenUML registration`, async ({
    page,
  }) => {
    await openDocument(page, `${ordinaryDocumentPath}?mode=${mode}`);
    const before = await readHostStyles(page);

    await page.goto(`${documentPath}?mode=${mode}`);
    await page.waitForFunction(
      () =>
        document.querySelector(".m2h-mermaid-frame .mermaid > svg") !== null,
    );
    const after = await readHostStyles(page);

    expect(after).toEqual(before);
    expect(after.oversizedHeadStyleCount).toBe(0);
  });
}

test("does not retain ZenUML host styles after in-app navigation", async ({
  page,
}) => {
  await openDocument(page, `${ordinaryDocumentPath}?mode=light`);
  const ordinary = await readHostStyles(page);

  await page.goto(`${documentPath}?mode=light`);
  await page.waitForFunction(
    () => document.querySelector(".m2h-mermaid-frame .mermaid > svg") !== null,
  );
  await page
    .getByRole("button", { name: "滚动恢复回归文档，scroll.md" })
    .click();
  await expect(page).toHaveURL(/\/doc\/scroll\.md\?mode=light$/);
  await page.waitForFunction(() =>
    document
      .querySelector(".markdown-body h1")
      ?.textContent?.includes("滚动恢复"),
  );

  expect(await readHostStyles(page)).toEqual(ordinary);
});

test("renders sixteen ZenUML diagrams without repeated global styles", async ({
  page,
}) => {
  await openDocument(page, `${ordinaryDocumentPath}?mode=dark`);
  const ordinary = await readHostStyles(page);

  await page.goto(`${stressDocumentPath}?mode=dark`);
  await expect(page.locator(".m2h-mermaid-frame .mermaid > svg")).toHaveCount(
    16,
  );
  const rendered = await readHostStyles(page);

  expect(rendered).toEqual(ordinary);
  expect(rendered.oversizedHeadStyleCount).toBe(0);
  await expect(
    page.locator(
      '.m2h-mermaid-frame .mermaid > svg[data-m2h-zenuml-theme="dark"]',
    ),
  ).toHaveCount(16);
});
