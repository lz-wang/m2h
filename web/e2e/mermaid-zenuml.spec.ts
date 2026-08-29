import { expect, test, type Page } from "@playwright/test";

// Real-browser regression for the ZenUML external diagram. Mermaid Core does
// not know the `zenuml` diagram type: without the vendored plugin registered
// through registerExternalDiagrams, a perfectly valid diagram degrades into
// Mermaid's "Syntax error in text" fallback. The unit tests mock the plugin
// module — only this spec executes the real load → register → initialize →
// render chain against the /runtime/mermaid-zenuml/ assets the Go binary
// embeds.

const documentPath = "/doc/mermaid-zenuml.md";

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
});
