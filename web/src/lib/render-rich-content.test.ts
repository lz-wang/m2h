import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { runInThisContext } from "node:vm";
import { waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type {
  MathAutoRenderer,
  MathAutoRenderOptions,
  MermaidRenderResult,
  MermaidRuntime,
  VegaEmbedOptions,
  VegaEmbedResult,
  VegaEmbedRuntime,
} from "./runtime-loader";

interface MermaidRunOptions {
  nodes: HTMLElement[];
  suppressErrors: boolean;
}

const mermaidMock = vi.hoisted(() => ({
  initialize: vi.fn<(config: unknown) => void>(),
  registerExternalDiagrams: vi.fn(async () => {}),
  run: vi.fn<(options: MermaidRunOptions) => Promise<void>>(async () => {}),
  render: vi.fn<(id: string, text: string) => Promise<MermaidRenderResult>>(
    async () => ({ svg: '<svg data-mock="mermaid"></svg>' }),
  ),
}));

const renderMathInElementMock = vi.hoisted(() =>
  vi.fn<(element: HTMLElement, options?: MathAutoRenderOptions) => void>(),
);

const loadMermaidMock = vi.hoisted(() =>
  vi.fn(async (): Promise<MermaidRuntime> => mermaidMock),
);

const ensureZenUMLRegisteredMock = vi.hoisted(() => vi.fn(async () => {}));

const loadKatexMock = vi.hoisted(() =>
  vi.fn(async (): Promise<MathAutoRenderer> => renderMathInElementMock),
);

const loadTablesortMock = vi.hoisted(() => vi.fn());

// Mirrors the real vegaEmbed contract closely enough for the enhancement
// layer: the container's content is replaced wholesale with the rendered
// visual, and the resolved result exposes finalize for lifecycle tests.
const vegaEmbedMock = vi.hoisted(() =>
  vi.fn<
    (
      element: HTMLElement,
      spec: object,
      options: VegaEmbedOptions,
    ) => Promise<VegaEmbedResult>
  >(async (element: HTMLElement) => {
    element.innerHTML = '<svg data-mock="vega-lite"></svg>';
    return { view: {}, finalize: vi.fn() };
  }),
);

const loadVegaLiteMock = vi.hoisted(() =>
  vi.fn(async (): Promise<VegaEmbedRuntime> => vegaEmbedMock),
);

vi.mock("./runtime-loader", () => ({
  loadMermaid: loadMermaidMock,
  loadKatex: loadKatexMock,
  loadTablesort: loadTablesortMock,
  ensureZenUMLRegistered: ensureZenUMLRegisteredMock,
  loadVegaLite: loadVegaLiteMock,
}));

// The vendored tablesort bundles, concatenated in the same order the runtime
// loader injects them so comparator registration priority matches production.
const TABLESORT_BUNDLE = [
  "tablesort.min.js",
  "tablesort.date.js",
  "tablesort.dotsep.js",
  "tablesort.filesize.js",
  "tablesort.monthname.js",
  "tablesort.number.js",
]
  .map((name) =>
    readFileSync(
      resolve(
        dirname(fileURLToPath(import.meta.url)),
        "../../../internal/assets/rich",
        name,
      ),
      "utf8",
    ),
  )
  .join("\n");

// Evaluating the bundles in the shared global context attaches
// window.Tablesort exactly as a <script> tag would, so the sorting assertions
// below run against the same runtime the document server serves the WebUI.
function installTablesortRuntime(): void {
  runInThisContext(TABLESORT_BUNDLE, { filename: "tablesort-bundle.js" });
}

describe("renderRichContent", () => {
  beforeEach(() => {
    // Each test re-imports the module so the lazy mermaid singleton resets.
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    loadTablesortMock.mockClear();
    loadVegaLiteMock.mockClear();
    vegaEmbedMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
    vegaEmbedMock.mockImplementation(async (element: HTMLElement) => {
      element.innerHTML = '<svg data-mock="vega-lite"></svg>';
      return { view: {}, finalize: vi.fn() };
    });
  });

  it("replaces mermaid code blocks with rendered SVG via mermaid.render", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    const container = root.querySelector("div.mermaid");
    expect(container).not.toBeNull();
    expect(container?.innerHTML).toContain('data-mock="mermaid"');
    expect(root.querySelector("pre")).toBeNull();
    expect(root.querySelector("code.language-mermaid")).toBeNull();
    // Mermaid blocks never get a frame either: their pre dies with the swap,
    // so an eagerly-created frame would linger as an empty wrapper.
    expect(root.querySelector(".m2h-code-frame")).toBeNull();

    // The legacy mermaid.run path is gone; each diagram renders offscreen via
    // mermaid.render with the decoded source text, then swaps in atomically.
    expect(mermaidMock.run).not.toHaveBeenCalled();
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
    const [id, text] = mermaidMock.render.mock.calls[0] ?? [];
    expect(id).toMatch(/^m2h-mermaid-\d+$/);
    expect(text).toBe("graph TD\nA-->B");
  });

  it("leaves non-mermaid code blocks untouched and skips mermaid.run", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<pre><code class="language-go">func main()</code></pre>';

    await renderRichContent(root, "light");

    expect(root.querySelector("pre")).not.toBeNull();
    expect(root.querySelector("div.mermaid")).toBeNull();
    expect(loadMermaidMock).not.toHaveBeenCalled();
    expect(mermaidMock.render).not.toHaveBeenCalled();
    expect(
      root.querySelector<HTMLButtonElement>(".m2h-code-copy"),
    ).not.toBeNull();
  });

  it("does not load the mermaid runtime without diagram blocks", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<p>plain text and $math$ only</p>";

    await renderRichContent(root, "light");

    expect(loadMermaidMock).not.toHaveBeenCalled();
    expect(loadKatexMock).toHaveBeenCalledTimes(1);
  });

  it("does not load the KaTeX runtime without math delimiters", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    expect(loadMermaidMock).toHaveBeenCalledTimes(1);
    expect(loadKatexMock).not.toHaveBeenCalled();
  });

  it("loads no runtime for a plain document", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<h2>heading</h2><p>paragraph without delimiters</p>";

    await renderRichContent(root, "light");

    expect(loadMermaidMock).not.toHaveBeenCalled();
    expect(loadKatexMock).not.toHaveBeenCalled();
  });

  it("adds a copy control that uses execCommand on HTTP", async () => {
    const restoreExecCommand = replaceProperty(
      document,
      "execCommand",
      vi.fn(() => true),
    );
    try {
      const { renderRichContent } = await import("./render-rich-content");
      const root = document.createElement("div");
      root.innerHTML =
        '<pre><code class="language-go">func main()</code></pre>';

      await renderRichContent(root, "light");

      const button = root.querySelector<HTMLButtonElement>(".m2h-code-copy");
      if (button === null) {
        throw new Error("code copy button was not added");
      }
      // DOM contract: the overlay control lives on the frame wrapping the
      // pre — never inside the pre's scrollport, whose scrolling content
      // would carry an absolutely-positioned child away with it.
      const pre = root.querySelector("pre");
      const frame = root.querySelector<HTMLElement>(".m2h-code-frame");
      expect(frame).not.toBeNull();
      expect(pre?.parentElement).toBe(frame);
      expect(button.parentElement).toBe(frame);
      expect(button.parentElement).not.toBe(pre);
      expect(pre?.querySelector(":scope > .m2h-code-copy")).toBeNull();
      expect(button.type).toBe("button");
      expect(button.getAttribute("aria-label")).toBe("复制代码");
      expect(button.title).toBe("复制代码");
      expect(button.textContent).toBe("");
      expect(button.querySelector('svg[aria-hidden="true"]')).not.toBeNull();

      button.click();
      await waitFor(() =>
        expect(button.getAttribute("aria-label")).toBe("代码已复制"),
      );
      expect(button.title).toBe("已复制");
      expect(document.execCommand).toHaveBeenCalledWith("copy");
    } finally {
      restoreExecCommand();
    }
  });

  it("uses Clipboard API in a secure context and falls back after a rejection", async () => {
    const restoreSecureContext = replaceProperty(
      window,
      "isSecureContext",
      true,
    );
    const writeText = vi
      .fn<(value: string) => Promise<void>>()
      .mockRejectedValueOnce(new DOMException("denied", "NotAllowedError"))
      .mockResolvedValueOnce(undefined);
    const restoreClipboard = replaceProperty(navigator, "clipboard", {
      writeText,
    });
    const restoreExecCommand = replaceProperty(
      document,
      "execCommand",
      vi.fn(() => true),
    );
    try {
      const { renderRichContent } = await import("./render-rich-content");
      const root = document.createElement("div");
      root.innerHTML =
        "<pre><code>first</code></pre><pre><code>second</code></pre>";

      await renderRichContent(root, "light");

      const buttons =
        root.querySelectorAll<HTMLButtonElement>(".m2h-code-copy");
      buttons[0]?.click();
      await waitFor(() =>
        expect(buttons[0]?.getAttribute("aria-label")).toBe("代码已复制"),
      );
      expect(document.execCommand).toHaveBeenCalledWith("copy");

      buttons[1]?.click();
      await waitFor(() =>
        expect(buttons[1]?.getAttribute("aria-label")).toBe("代码已复制"),
      );
      expect(writeText).toHaveBeenCalledWith("first");
      expect(writeText).toHaveBeenCalledWith("second");
      expect(document.execCommand).toHaveBeenCalledTimes(1);
    } finally {
      restoreExecCommand();
      restoreClipboard();
      restoreSecureContext();
    }
  });

  it("skips mermaid code that is not wrapped in a pre element", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<code class="language-mermaid">graph TD</code>';

    await renderRichContent(root, "light");

    expect(mermaidMock.run).not.toHaveBeenCalled();
    expect(root.querySelector("code.language-mermaid")).not.toBeNull();
  });

  it("hands the root to KaTeX with $$ before $ and throwOnError disabled", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.textContent = "$E=mc^2$";

    await renderRichContent(root, "light");

    expect(renderMathInElementMock).toHaveBeenCalledTimes(1);
    const [element, options] = renderMathInElementMock.mock.calls[0];
    expect(element).toBe(root);
    if (options === undefined) {
      throw new Error("renderMathInElement was called without options");
    }
    expect(options.throwOnError).toBe(false);
    expect(options.ignoredClasses).toEqual(["m2h-literal-dollar"]);

    const lefts =
      options.delimiters?.map(
        (delimiter: { left: string }) => delimiter.left,
      ) ?? [];
    expect(lefts.indexOf("$$")).toBeLessThan(lefts.indexOf("$"));
    expect(lefts).toEqual(["$$", "\\[", "\\(", "$"]);
    expect(root.querySelector(".m2h-literal-dollar")).toBeNull();
  });

  it("keeps currency dollars literal while preserving valid inline math", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    const source =
      "第一次裸跑，20 分钟花了 $9，公式 $E=mc^2$，第二次 6 小时花了 $200。";
    root.textContent = source;

    await renderRichContent(root, "light");

    expect(renderMathInElementMock).toHaveBeenCalledTimes(1);
    expect(root.textContent).toBe(source);
    expect(
      Array.from(
        root.querySelectorAll(".m2h-literal-dollar"),
        (node) => node.textContent,
      ),
    ).toEqual(["$", "$"]);

    const [, options] = renderMathInElementMock.mock.calls[0];
    expect(options?.ignoredClasses).toContain("m2h-literal-dollar");
  });

  it("runs mermaid before KaTeX so math never scans diagram source", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const order: string[] = [];
    mermaidMock.render.mockImplementation(async () => {
      order.push("mermaid");
      return { svg: "<svg></svg>" };
    });
    renderMathInElementMock.mockImplementation(() => {
      order.push("katex");
    });

    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>$E=mc^2$</p>';

    await renderRichContent(root, "light");

    expect(order).toEqual(["mermaid", "katex"]);
  });

  it("initializes mermaid with the official light theme before running diagrams", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    expect(mermaidMock.initialize).toHaveBeenCalledWith({
      startOnLoad: false,
      securityLevel: "strict",
      theme: "default",
    });
  });

  it("re-initializes mermaid only when the resolved theme changes", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const lightRoot = document.createElement("div");
    lightRoot.innerHTML =
      '<pre><code class="language-mermaid">graph TD</code></pre>';
    await renderRichContent(lightRoot, "light");

    // Re-rendering the same light theme must not reconfigure Mermaid.
    const lightRepeat = document.createElement("div");
    lightRepeat.innerHTML =
      '<pre><code class="language-mermaid">graph TD</code></pre>';
    await renderRichContent(lightRepeat, "light");
    expect(mermaidMock.initialize).toHaveBeenCalledTimes(1);

    // Switching to dark flips the official theme and re-runs initialize so the
    // already-baked light SVGs regenerate in the dark palette.
    const darkRoot = document.createElement("div");
    darkRoot.innerHTML =
      '<pre><code class="language-mermaid">graph TD</code></pre>';
    await renderRichContent(darkRoot, "dark");

    expect(mermaidMock.initialize).toHaveBeenCalledTimes(2);
    expect(mermaidMock.initialize).toHaveBeenNthCalledWith(1, {
      startOnLoad: false,
      securityLevel: "strict",
      theme: "default",
    });
    expect(mermaidMock.initialize).toHaveBeenNthCalledWith(2, {
      startOnLoad: false,
      securityLevel: "strict",
      theme: "dark",
    });
  });

  it("skips KaTeX when a stale render is reported after mermaid resolves", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>$E=mc^2$</p>';

    // Mermaid runs (the mock resolves), but the render is no longer current, so
    // KaTeX must not scan the now-stale root.
    await renderRichContent(root, "light", () => false);

    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
    expect(renderMathInElementMock).not.toHaveBeenCalled();
  });

  it("still renders KaTeX when the freshness check reports current", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>$E=mc^2$</p>';

    await renderRichContent(root, "light", () => true);

    expect(renderMathInElementMock).toHaveBeenCalledTimes(1);
  });

  it("prepends a permalink anchor to every heading that has an id", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<h1 id="title">Title</h1><h2 id="section">Section</h2><h3>no id</h3>';

    await renderRichContent(root, "light");

    const h1Anchor = root.querySelector<HTMLAnchorElement>(
      "h1#title > .m2h-heading-anchor",
    );
    const h2Anchor = root.querySelector<HTMLAnchorElement>(
      "h2#section > .m2h-heading-anchor",
    );
    expect(h1Anchor).not.toBeNull();
    expect(h2Anchor).not.toBeNull();
    expect(h1Anchor?.getAttribute("href")).toBe("#title");
    expect(h1Anchor?.getAttribute("aria-hidden")).toBe("true");
    expect(h1Anchor?.title).toBe("此标题的永久链接");
    expect(h1Anchor?.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
    // The anchor is the first child so it sits to the left of the heading text.
    expect(root.querySelector("h1#title")?.firstElementChild).toBe(h1Anchor);
    // A heading without an id is left untouched.
    expect(root.querySelector("h3 > .m2h-heading-anchor")).toBeNull();
  });

  it("does not duplicate permalinks when enhancement runs twice", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<h2 id="install">Install</h2>';

    await renderRichContent(root, "light");
    await renderRichContent(root, "light");

    expect(
      root.querySelectorAll("h2#install > .m2h-heading-anchor"),
    ).toHaveLength(1);
  });
});

describe("collapsible code blocks", () => {
  beforeEach(() => {
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    loadTablesortMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
  });

  it("leaves blocks at or below the threshold unwrapped", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    // The last block carries the trailing "\n" fenced code typically ends
    // with: 25 content lines must still count as 25, not 26.
    root.innerHTML =
      `<pre><code class="language-go">${codeLines(24)}</code></pre>` +
      `<pre><code class="language-go">${codeLines(25)}</code></pre>` +
      `<pre><code class="language-go">${codeLines(25)}\n</code></pre>`;

    await renderRichContent(root, "light");

    expect(root.querySelectorAll(".m2h-code-block")).toHaveLength(0);
    expect(root.querySelectorAll(".m2h-code-toggle")).toHaveLength(0);
    // Short blocks still get their frame; the copy control rides on it.
    expect(root.querySelectorAll(".m2h-code-frame")).toHaveLength(3);
    expect(
      root.querySelectorAll(".m2h-code-frame > .m2h-code-copy"),
    ).toHaveLength(3);
    expect(root.querySelectorAll("pre > .m2h-code-copy")).toHaveLength(0);
  });

  it("collapses blocks above the threshold by default", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-go">${codeLines(26)}</code></pre>`;

    await renderRichContent(root, "light");

    const wrapper = root.querySelector<HTMLElement>(".m2h-code-block");
    expect(wrapper).not.toBeNull();
    expect(wrapper?.dataset.collapsible).toBe("true");
    expect(wrapper?.dataset.collapsed).toBe("true");
    expect(wrapper?.dataset.lineCount).toBe("26");

    // The fold is a modifier on the frame: one frame, one scrollport (the
    // pre), and the two overlay/controls (copy, toggle) as its children.
    expect(wrapper?.classList.contains("m2h-code-frame")).toBe(true);
    const pre = wrapper?.querySelector("pre");
    const copy = wrapper?.querySelector<HTMLElement>(":scope > .m2h-code-copy");
    const toggle = root.querySelector<HTMLButtonElement>(".m2h-code-toggle");
    expect(pre).not.toBeNull();
    expect(copy).not.toBeNull();
    expect(toggle).not.toBeNull();
    expect(pre?.parentElement).toBe(wrapper);
    expect(copy?.parentElement).toBe(wrapper);
    expect(toggle?.parentElement).toBe(wrapper);
    expect(toggle?.getAttribute("aria-controls")).toBe(pre?.id);
    expect(toggle?.getAttribute("aria-expanded")).toBe("false");
    expect(toggle?.textContent).toBe("展开代码（共26行）");
  });

  it("expands and re-collapses through the toggle", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-go">${codeLines(26)}</code></pre>`;

    await renderRichContent(root, "light");
    const wrapper = root.querySelector<HTMLElement>(".m2h-code-block");
    const toggle = root.querySelector<HTMLButtonElement>(".m2h-code-toggle");
    if (wrapper === null || toggle === null) {
      throw new Error("collapsible code block was not added");
    }

    toggle.click();
    expect(wrapper.dataset.collapsed).toBe("false");
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    expect(toggle.textContent).toBe("折叠代码");

    toggle.click();
    expect(wrapper.dataset.collapsed).toBe("true");
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(toggle.textContent).toBe("展开代码（共26行）");
  });

  it("labels the toggle with the full line count", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-go">${codeLines(127)}</code></pre>`;

    await renderRichContent(root, "light");

    expect(
      root.querySelector<HTMLElement>(".m2h-code-block")?.dataset.lineCount,
    ).toBe("127");
    expect(
      root.querySelector<HTMLButtonElement>(".m2h-code-toggle")?.textContent,
    ).toBe("展开代码（共127行）");
  });

  it("never folds mermaid source", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-mermaid">${codeLines(60)}</code></pre>`;

    await renderRichContent(root, "light");

    // The diagram renders and no collapse affordance is created for it.
    expect(root.querySelector("div.mermaid")).not.toBeNull();
    expect(root.querySelector(".m2h-code-block")).toBeNull();
    expect(root.querySelector(".m2h-code-toggle")).toBeNull();
  });

  it("does not stack a second wrapper on repeated enhancement", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-go">${codeLines(60)}</code></pre>`;

    await renderRichContent(root, "light");
    await renderRichContent(root, "light");

    expect(root.querySelectorAll(".m2h-code-block")).toHaveLength(1);
    expect(root.querySelectorAll(".m2h-code-toggle")).toHaveLength(1);
    expect(root.querySelectorAll(".m2h-code-frame")).toHaveLength(1);
  });

  it("keeps the copy control copying the complete source", async () => {
    let copiedValue = "";
    const restoreExecCommand = replaceProperty(
      document,
      "execCommand",
      vi.fn(() => {
        copiedValue =
          document.querySelector<HTMLTextAreaElement>(
            "textarea[aria-hidden='true']",
          )?.value ?? "";
        return true;
      }),
    );
    try {
      const { renderRichContent } = await import("./render-rich-content");
      const root = document.createElement("div");
      root.innerHTML = `<pre><code class="language-go">${codeLines(127)}</code></pre>`;

      await renderRichContent(root, "light");

      // Collapsing is presentation only: the DOM keeps the whole source …
      const code = root.querySelector("code");
      expect(countLines(code?.textContent ?? "")).toBe(127);
      // … and the copy control on the folded block's frame copies all of it.
      const copy = root.querySelector<HTMLButtonElement>(".m2h-code-copy");
      if (copy === null) {
        throw new Error("code copy button was not added");
      }
      copy.click();
      await waitFor(() =>
        expect(copy.getAttribute("aria-label")).toBe("代码已复制"),
      );
      expect(countLines(copiedValue)).toBe(127);
    } finally {
      restoreExecCommand();
    }
  });
});

describe("code line numbers", () => {
  beforeEach(() => {
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    loadTablesortMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
  });

  it("numbers every source line in a gutter beside the code", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    // Trailing "\n" is what fenced code characteristically carries: 3 source
    // lines must number as 1–3, not 1–4.
    root.innerHTML = `<pre><code class="language-go">a\nb\nc\n</code></pre>`;

    await renderRichContent(root, "light");

    const gutter = root.querySelector<HTMLElement>(
      "pre > .m2h-code-line-numbers",
    );
    expect(gutter).not.toBeNull();
    expect(
      root.querySelector("pre")?.classList.contains("m2h-code-with-lines"),
    ).toBe(true);
    expect(gutter?.getAttribute("aria-hidden")).toBe("true");
    expect(gutter?.textContent).toBe("123");
    // The gutter sits before the code and never inside it, so the source —
    // and therefore the copy control — stays untouched by the numbers.
    expect(gutter?.nextElementSibling?.tagName).toBe("CODE");
    expect(root.querySelector("code")?.textContent).toBe("a\nb\nc\n");
  });

  it("keeps line numbers out of the copied source", async () => {
    let copiedValue = "";
    const restoreExecCommand = replaceProperty(
      document,
      "execCommand",
      vi.fn(() => {
        copiedValue =
          document.querySelector<HTMLTextAreaElement>(
            "textarea[aria-hidden='true']",
          )?.value ?? "";
        return true;
      }),
    );
    try {
      const { renderRichContent } = await import("./render-rich-content");
      const root = document.createElement("div");
      root.innerHTML = `<pre><code class="language-go">a\nb\n</code></pre>`;

      await renderRichContent(root, "light");
      const copy = root.querySelector<HTMLButtonElement>(".m2h-code-copy");
      if (copy === null) {
        throw new Error("code copy button was not added");
      }

      copy.click();
      await waitFor(() =>
        expect(copy.getAttribute("aria-label")).toBe("代码已复制"),
      );
      expect(copiedValue).toBe("a\nb\n");
    } finally {
      restoreExecCommand();
    }
  });

  it("numbers an empty code block as a single line", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<pre><code></code></pre><pre><code>\n</code></pre>";

    await renderRichContent(root, "light");

    const gutters = root.querySelectorAll<HTMLElement>(
      "pre > .m2h-code-line-numbers",
    );
    expect(gutters).toHaveLength(2);
    expect(gutters[0]?.textContent).toBe("1");
    expect(gutters[1]?.textContent).toBe("1");
  });

  it("never numbers mermaid source", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>' +
      `<pre><code class="language-go">x\n</code></pre>`;

    await renderRichContent(root, "light");

    expect(root.querySelector("div.mermaid")).not.toBeNull();
    expect(root.querySelectorAll("pre > .m2h-code-line-numbers")).toHaveLength(
      1,
    );
  });

  it("does not stack a second gutter on repeated enhancement", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-go">a\nb\nc\n</code></pre>`;

    await renderRichContent(root, "light");
    await renderRichContent(root, "light");

    expect(
      root.querySelectorAll("pre > .m2h-code-line-numbers > span"),
    ).toHaveLength(3);
  });

  it("numbers a collapsible block with the same count the toggle announces", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-go">${codeLines(30)}</code></pre>`;

    await renderRichContent(root, "light");

    // The gutter and the fold share one line-count algorithm: they can never
    // disagree about how many lines a block has.
    expect(
      root.querySelectorAll(
        ".m2h-code-block pre > .m2h-code-line-numbers > span",
      ),
    ).toHaveLength(30);
    expect(
      root.querySelector<HTMLButtonElement>(".m2h-code-toggle")?.textContent,
    ).toBe("展开代码（共30行）");
  });
});

function codeLines(count: number): string {
  return Array.from({ length: count }, (_, index) => `line ${index + 1}`).join(
    "\n",
  );
}

function countLines(source: string): number {
  if (source === "") {
    return 0;
  }
  return source.split("\n").length - (source.endsWith("\n") ? 1 : 0);
}

describe("image lightbox triggers", () => {
  beforeEach(() => {
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    loadTablesortMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
  });

  // The table-sort cross-feature test below installs the real tablesort
  // runtime; drop it again so later describes never see a stale bundle.
  afterEach(() => {
    delete window.Tablesort;
  });

  it("wraps a plain image in a frame with a magnifier trigger", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<p><img src="/a.png" alt="A"></p>';

    await renderRichContent(root, "light");

    const frame = root.querySelector<HTMLElement>(".m2h-image-frame");
    const image = root.querySelector<HTMLImageElement>("img");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-lightbox-trigger",
    );
    expect(frame).not.toBeNull();
    expect(image).not.toBeNull();
    expect(button).not.toBeNull();
    expect(image?.parentElement).toBe(frame);
    expect(button?.parentElement).toBe(frame);
    expect(image?.dataset.m2hLightboxItem).toBe("true");
    expect(button?.type).toBe("button");
    expect(button?.getAttribute("aria-label")).toBe("查看大图");
    expect(button?.title).toBe("查看大图");
    expect(button?.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
  });

  it("adds an alt-text name tooltip to a framed image", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<p><img src="/a.png" alt="architecture"></p>';

    await renderRichContent(root, "light");

    // The Markdown image name rides on the frame as a hover tooltip, hidden
    // from the accessibility tree: the <img> alt already carries the name.
    const frame = root.querySelector<HTMLElement>(".m2h-image-frame");
    const tooltip = frame?.querySelector<HTMLElement>(
      ":scope > .m2h-image-name-tooltip",
    );
    expect(tooltip).not.toBeNull();
    expect(tooltip?.textContent).toBe("architecture");
    expect(tooltip?.getAttribute("aria-hidden")).toBe("true");
    // The tooltip sits between the image and the trigger, all frame children.
    expect(tooltip?.previousElementSibling?.tagName).toBe("IMG");
    expect(tooltip?.nextElementSibling?.classList).toContain(
      "m2h-lightbox-trigger",
    );
  });

  it("omits the name tooltip when the image has no alt text", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><img src="/a.png" alt=""></p><p><img src="/b.png"></p>';

    await renderRichContent(root, "light");

    expect(root.querySelectorAll(".m2h-image-frame")).toHaveLength(2);
    expect(root.querySelectorAll(".m2h-image-name-tooltip")).toHaveLength(0);
  });

  it("does not stack a second tooltip on repeated enhancement", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<p><img src="/a.png" alt="A"></p>';

    await renderRichContent(root, "light");
    await renderRichContent(root, "light");

    expect(root.querySelectorAll(".m2h-image-name-tooltip")).toHaveLength(1);
  });

  it("frames images in document order without baking in position indexes", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><img src="1.png"></p><p><img src="2.png"></p><p><img src="3.png"></p>';

    await renderRichContent(root, "light");

    const images = Array.from(root.querySelectorAll<HTMLImageElement>("img"));
    const frames = Array.from(
      root.querySelectorAll<HTMLElement>(".m2h-image-frame"),
    );
    expect(frames).toHaveLength(3);
    // Document order pairs each image with its own frame, and the DOM carries
    // no position index for a click-time lookup to go stale against (a
    // sortable table reorders rows after this pass).
    expect(images.map((image) => image.closest(".m2h-image-frame"))).toEqual(
      frames,
    );
    expect(root.querySelector("[data-m2h-lightbox-index]")).toBeNull();
    expect(root.querySelectorAll(".m2h-lightbox-trigger")).toHaveLength(3);
  });

  it("does not stack a second frame on repeated enhancement", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><img src="1.png"></p><p><img src="2.png"></p><p><img src="3.png"></p>';

    await renderRichContent(root, "light");
    await renderRichContent(root, "light");

    expect(root.querySelectorAll("img")).toHaveLength(3);
    expect(root.querySelectorAll(".m2h-image-frame")).toHaveLength(3);
    expect(root.querySelectorAll(".m2h-lightbox-trigger")).toHaveLength(3);
    // No frame nested inside another frame either.
    expect(
      root.querySelectorAll(".m2h-image-frame .m2h-image-frame"),
    ).toHaveLength(0);
  });

  it("wraps the anchor of a linked image, keeping the button outside the link", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<p><a href="/target"><img src="/a.png" alt="A"></a></p>';

    await renderRichContent(root, "light");

    const frame = root.querySelector<HTMLElement>(".m2h-image-frame");
    const anchor = root.querySelector<HTMLAnchorElement>("a");
    const image = root.querySelector<HTMLImageElement>("img");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-lightbox-trigger",
    );
    expect(frame).not.toBeNull();
    expect(anchor?.parentElement).toBe(frame);
    expect(image?.parentElement).toBe(anchor);
    expect(button?.parentElement).toBe(frame);
    // Interactive content must never nest: the trigger lives beside the <a>.
    expect(anchor?.querySelector("button")).toBeNull();
    expect(anchor?.getAttribute("href")).toBe("/target");
  });

  it("wraps the anchor of an image nested in a span, keeping the button outside the link", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><a href="/target"><span><img src="/a.png" alt="A"></span></a></p>';

    await renderRichContent(root, "light");

    // The sole-image anchor is found through closest("a") even when the image
    // sits in a wrapper span, so the frame wraps the anchor — not the image —
    // and no button lands inside the link.
    const frame = root.querySelector<HTMLElement>(".m2h-image-frame");
    const anchor = root.querySelector<HTMLAnchorElement>("a");
    expect(anchor?.parentElement).toBe(frame);
    expect(anchor?.querySelector("button")).toBeNull();
    expect(anchor?.querySelector("img")?.dataset.m2hLightboxItem).toBe("true");
  });

  it("leaves a multi-image anchor untouched", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><a href="/target"><img src="1.png"><img src="2.png"></a></p>';

    await renderRichContent(root, "light");

    // Framing either image would nest the trigger button inside the <a>
    // (invalid interactive content, and Enter would follow the link), so the
    // raw-HTML structure wins: no frame, no trigger, no lightbox marker.
    const anchor = root.querySelector<HTMLAnchorElement>("a");
    expect(anchor?.querySelector("button")).toBeNull();
    expect(
      anchor?.querySelector("img")?.closest(".m2h-image-frame"),
    ).toBeNull();
    expect(root.querySelectorAll(".m2h-image-frame")).toHaveLength(0);
    expect(root.querySelectorAll(".m2h-lightbox-trigger")).toHaveLength(0);
    expect(
      root.querySelectorAll('[data-m2h-lightbox-item="true"]'),
    ).toHaveLength(0);
  });

  it("leaves images alone when one anchor spans several wrappers", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><a href="/target"><span><img src="1.png"></span><span><img src="2.png"></span></a></p>';

    await renderRichContent(root, "light");

    // The multi-image anchor is detected through closest("a"), so wrapping
    // each image in its own span must not smuggle a button into the link.
    expect(root.querySelector("a")?.querySelector("button")).toBeNull();
    expect(root.querySelectorAll(".m2h-lightbox-trigger")).toHaveLength(0);
  });

  it("wraps the picture of a responsive image, keeping source selection", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><picture><source srcset="dark.png" media="(prefers-color-scheme: dark)"><img src="light.png" alt="L"></picture></p>';

    await renderRichContent(root, "light");

    // The frame wraps the <picture> itself: the <img> must stay the
    // picture's direct child next to the <source>, or source selection
    // breaks.
    const frame = root.querySelector<HTMLElement>(".m2h-image-frame");
    const picture = root.querySelector<HTMLPictureElement>("picture");
    const image = root.querySelector<HTMLImageElement>("img");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-lightbox-trigger",
    );
    expect(picture?.parentElement).toBe(frame);
    expect(image?.parentElement).toBe(picture);
    expect(picture?.querySelector("source")?.nextElementSibling).toBe(image);
    expect(button?.parentElement).toBe(frame);
    expect(image?.dataset.m2hLightboxItem).toBe("true");
  });

  it("wraps the anchor of a linked picture, keeping the button outside the link", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<p><a href="/target"><picture><source srcset="dark.png" media="(prefers-color-scheme: dark)"><img src="light.png" alt="L"></picture></a></p>';

    await renderRichContent(root, "light");

    const frame = root.querySelector<HTMLElement>(".m2h-image-frame");
    const anchor = root.querySelector<HTMLAnchorElement>("a");
    const picture = root.querySelector<HTMLPictureElement>("picture");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-lightbox-trigger",
    );
    expect(anchor?.parentElement).toBe(frame);
    expect(picture?.parentElement).toBe(anchor);
    expect(picture?.querySelector("img")?.parentElement).toBe(picture);
    expect(button?.parentElement).toBe(frame);
    expect(anchor?.querySelector("button")).toBeNull();
  });

  it("frames a rendered mermaid diagram with exactly one shared trigger", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    // The diagram pass owns its frame: a stable wrapper around the container
    // (which mermaid rewrites on every theme switch) carrying the shared
    // trigger, with the lightbox marker on the container itself.
    const frame = root.querySelector<HTMLElement>(".m2h-mermaid-frame");
    const container = root.querySelector<HTMLElement>("div.mermaid");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-mermaid-frame > .m2h-lightbox-trigger",
    );
    expect(frame).not.toBeNull();
    expect(container?.parentElement).toBe(frame);
    expect(button?.parentElement).toBe(frame);
    expect(container?.dataset.m2hLightboxItem).toBe("true");
    expect(button?.type).toBe("button");
    expect(button?.getAttribute("aria-label")).toBe("查看 Mermaid 图表");
    expect(button?.title).toBe("查看 Mermaid 图表");
    // The successful paint is what unhides the trigger: availability follows
    // the SVG's presence, never the frame's existence.
    expect(button?.hidden).toBe(false);
    expect(button?.querySelector('svg[aria-hidden="true"]')).not.toBeNull();
    // The image pass must not have added a second trigger: mermaid never
    // appears as an <img>, so exactly one button lives in the frame.
    expect(root.querySelectorAll(".m2h-lightbox-trigger")).toHaveLength(1);
    expect(root.querySelectorAll("img")).toHaveLength(0);
  });

  it("withholds the lightbox marker and trigger while a diagram has no SVG", async () => {
    // The paint never resolves: the frame exists, but no SVG ever lands.
    mermaidMock.render.mockImplementation(
      () => new Promise(() => {}) as Promise<MermaidRenderResult>,
    );
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    const pending = renderRichContent(root, "light");
    await waitFor(() => {
      expect(root.querySelector(".m2h-mermaid-frame")).not.toBeNull();
    });
    const frame = root.querySelector<HTMLElement>(".m2h-mermaid-frame");
    const container = root.querySelector<HTMLElement>("div.mermaid");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-mermaid-frame > .m2h-lightbox-trigger",
    );
    // Initial state: the frame and source text are there, but there is
    // nothing to enlarge yet — no marker, trigger hidden. The pending paint
    // can never settle this state, so the assertions stay stable.
    expect(frame).not.toBeNull();
    expect(container?.textContent).toContain("graph TD");
    expect(container?.dataset.m2hLightboxItem).toBeUndefined();
    expect(button?.hidden).toBe(true);
    void pending;
  });

  it("keeps the trigger hidden and marker absent after a failed first render", async () => {
    mermaidMock.render.mockRejectedValue(new Error("invalid syntax"));
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">not a diagram [[</code></pre>';

    await renderRichContent(root, "light");

    // The failure is isolated to this diagram: the frame stays with its
    // source text, but the lightbox is not offered.
    const container = root.querySelector<HTMLElement>("div.mermaid");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-mermaid-frame > .m2h-lightbox-trigger",
    );
    expect(container?.querySelector("svg")).toBeNull();
    expect(container?.textContent).toContain("not a diagram");
    expect(container?.dataset.m2hLightboxItem).toBeUndefined();
    expect(button?.hidden).toBe(true);
  });

  it("keeps the lightbox available when a theme repaint fails on an existing SVG", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");
    const container = root.querySelector<HTMLElement>("div.mermaid");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-mermaid-frame > .m2h-lightbox-trigger",
    );
    if (container === null || button === null) {
      throw new Error("mermaid frame was not created");
    }
    expect(container.dataset.m2hLightboxItem).toBe("true");
    expect(button.hidden).toBe(false);

    // The dark repaint throws: the old SVG stays in place, so the lightbox
    // keeps working — availability follows the SVG that is still there.
    mermaidMock.render.mockRejectedValue(new Error("dark render failed"));
    await rerenderMermaid(root, "dark");

    expect(container.innerHTML).toContain('data-mock="mermaid"');
    expect(container.dataset.m2hLightboxItem).toBe("true");
    expect(button.hidden).toBe(false);
  });

  it("keeps the mermaid frame, marker and trigger across a theme re-render", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");
    const frame = root.querySelector<HTMLElement>(".m2h-mermaid-frame");
    const container = root.querySelector<HTMLElement>("div.mermaid");
    const button = root.querySelector<HTMLButtonElement>(
      ".m2h-mermaid-frame > .m2h-lightbox-trigger",
    );
    if (frame === null || container === null || button === null) {
      throw new Error("mermaid frame was not created");
    }
    // Focus only lands on connected elements, so the root joins the document
    // for this test (and leaves again in the finally below).
    document.body.append(root);
    button.focus();
    mermaidMock.render.mockClear();
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="dark"></svg>',
    });

    try {
      await rerenderMermaid(root, "dark");

      // Only the SVG inside the container is replaced; the frame, the marker,
      // the trigger — and focus on it — all survive the repaint, and no
      // second trigger stacks.
      expect(container.innerHTML).toContain('data-mock="dark"');
      expect(root.querySelectorAll(".m2h-mermaid-frame")).toHaveLength(1);
      expect(root.querySelectorAll(".m2h-lightbox-trigger")).toHaveLength(1);
      expect(container.dataset.m2hLightboxItem).toBe("true");
      expect(document.activeElement).toBe(button);
    } finally {
      root.remove();
    }
  });

  // Cross-feature regression: Tablesort really moves <tr> elements, so the
  // image a trigger addresses must be resolved from the DOM order at click
  // time. With a position index baked in at enhancement time, this test opens
  // the wrong image after the first sort.
  it("resolves the clicked image after a table sort reorders rows", async () => {
    installTablesortRuntime();
    loadTablesortMock.mockImplementation(async () => {
      const Tablesort = window.Tablesort;
      if (Tablesort === undefined) {
        throw new Error("tablesort runtime is not installed");
      }
      return Tablesort;
    });
    const { renderRichContent } = await import("./render-rich-content");
    const { collectLightboxState } = await import("./document-lightbox");
    const root = document.createElement("div");
    root.innerHTML = `<table><thead><tr><th>Name</th><th>Image</th></tr></thead>
      <tbody>
        <tr><td>Alpha</td><td><img src="/a.png" alt="A"></td></tr>
        <tr><td>Beta</td><td><img src="/b.png" alt="B"></td></tr>
      </tbody></table>`;

    await renderRichContent(root, "light");

    // Sort by name descending: the Beta row — and its b.png — moves first.
    const header = root.querySelector<HTMLTableCellElement>("thead th");
    header?.click();
    header?.click();
    expect(
      Array.from(root.querySelectorAll("tbody td:first-child")).map(
        (cell) => cell.textContent,
      ),
    ).toEqual(["Beta", "Alpha"]);

    // The trigger press resolves through the frame to its own image, which is
    // then indexed against the current DOM order: b.png is now image 0.
    const frame = root.querySelector<HTMLElement>("tbody .m2h-image-frame");
    const selected = frame?.querySelector<HTMLImageElement>(
      '[data-m2h-lightbox-item="true"]',
    );
    const state =
      selected === undefined || selected === null
        ? null
        : collectLightboxState(root, selected);
    expect(state?.index).toBe(0);
    expect(
      (state?.items ?? []).map((item) => new URL(item.src).pathname),
    ).toEqual(["/b.png", "/a.png"]);
  });
});

describe("rerenderMermaid", () => {
  beforeEach(() => {
    // Each test re-imports the module so the lazy mermaid singleton and the
    // mermaidSources WeakMap reset between cases.
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    loadTablesortMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
  });

  it("regenerates only mermaid diagrams in the new theme, leaving the body intact", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>keep me</p>';

    await renderRichContent(root, "light");

    const container = root.querySelector<HTMLDivElement>("div.mermaid");
    if (container === null) {
      throw new Error("mermaid container was not created");
    }
    const paragraph = root.querySelector("p");
    if (paragraph === null) {
      throw new Error("paragraph was not rendered");
    }
    mermaidMock.render.mockClear();
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="dark"></svg>',
    });

    await rerenderMermaid(root, "dark");

    expect(mermaidMock.initialize).toHaveBeenCalledWith(
      expect.objectContaining({ theme: "dark" }),
    );
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
    expect(container.innerHTML).toContain('data-mock="dark"');
    // The body DOM is not rebuilt: the paragraph keeps its node identity and
    // KaTeX / copy-button enhancement never runs again.
    expect(root.querySelector("p")).toBe(paragraph);
    expect(renderMathInElementMock).not.toHaveBeenCalled();
  });

  it("does not load the mermaid runtime when there are no diagrams", async () => {
    const { rerenderMermaid } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<p>plain document</p>";

    await rerenderMermaid(root, "dark");

    expect(loadMermaidMock).not.toHaveBeenCalled();
    expect(mermaidMock.render).not.toHaveBeenCalled();
  });

  it("keeps the previous SVG when a theme re-render throws", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    const container = root.querySelector<HTMLDivElement>("div.mermaid");
    if (container === null) {
      throw new Error("mermaid container was not created");
    }
    mermaidMock.render.mockClear();
    mermaidMock.render.mockRejectedValue(new Error("boom"));

    await rerenderMermaid(root, "dark");

    // The light SVG stays in place rather than flashing back to source text.
    expect(container.innerHTML).toContain('data-mock="mermaid"');
  });

  it("aborts the remaining diagrams once the render is no longer current", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA</code></pre><pre><code class="language-mermaid">graph TD\nB</code></pre>';

    await renderRichContent(root, "light");
    mermaidMock.render.mockClear();

    await rerenderMermaid(root, "dark", () => false);

    // The first diagram renders, then the freshness check aborts before the
    // second target is painted.
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
  });
});

describe("mermaid external diagrams (ZenUML)", () => {
  beforeEach(() => {
    // Each test re-imports the module so the lazy mermaid singleton and the
    // ZenUML registration state reset between cases.
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    loadTablesortMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
    ensureZenUMLRegisteredMock.mockReset();
    ensureZenUMLRegisteredMock.mockResolvedValue(undefined);
  });

  it("does not fetch the ZenUML plugin for plain Mermaid diagrams", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    expect(loadMermaidMock).toHaveBeenCalledTimes(1);
    expect(ensureZenUMLRegisteredMock).not.toHaveBeenCalled();
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
  });

  it("does not treat prose mentioning zenuml as a ZenUML diagram", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    // Only a leading zenuml keyword identifies the diagram type; the word
    // appearing elsewhere is ordinary diagram content.
    root.innerHTML =
      '<pre><code class="language-mermaid">flowchart TD\nA[zenuml inside a label]</code></pre>';

    await renderRichContent(root, "light");

    expect(ensureZenUMLRegisteredMock).not.toHaveBeenCalled();
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
  });

  it("registers ZenUML once in load → register → initialize → render order", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">zenuml\n    Alice->Bob: Hello</code></pre>';

    await renderRichContent(root, "light");

    expect(ensureZenUMLRegisteredMock).toHaveBeenCalledTimes(1);
    expect(ensureZenUMLRegisteredMock).toHaveBeenCalledWith(mermaidMock);
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
    // Mermaid's integration order: the plugin must be known before
    // initialize configures the runtime and before any diagram renders.
    expect(loadMermaidMock.mock.invocationCallOrder[0]).toBeLessThan(
      ensureZenUMLRegisteredMock.mock.invocationCallOrder[0],
    );
    expect(ensureZenUMLRegisteredMock.mock.invocationCallOrder[0]).toBeLessThan(
      mermaidMock.initialize.mock.invocationCallOrder[0],
    );
    expect(mermaidMock.initialize.mock.invocationCallOrder[0]).toBeLessThan(
      mermaidMock.render.mock.invocationCallOrder[0],
    );
  });

  it("keeps the light SVG intact and adds a scoped dark ZenUML palette on rerender", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    mermaidMock.render.mockResolvedValue({
      svg: `<svg data-mock="zenuml"><defs><style data-upstream="true">
        .participant-box { fill: #ffffff; stroke: #666; }
        .participant-label { fill: #222; }
      </style></defs><rect class="participant-box"></rect><text class="participant-label">Alice</text></svg>`,
    });
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">zenuml\n    Alice->Bob: Hello</code></pre>';

    await renderRichContent(root, "light");

    const lightSVG = root.querySelector<SVGSVGElement>(".mermaid > svg");
    expect(lightSVG?.dataset.m2hZenumlTheme).toBe("light");
    expect(
      lightSVG?.querySelector('[data-m2h-zenuml-theme-style="dark"]'),
    ).toBeNull();
    expect(lightSVG?.querySelector('[data-upstream="true"]')).not.toBeNull();

    await rerenderMermaid(root, "dark");

    const darkSVG = root.querySelector<SVGSVGElement>(".mermaid > svg");
    const darkStyle = darkSVG?.querySelector(
      '[data-m2h-zenuml-theme-style="dark"]',
    );
    expect(darkSVG).not.toBe(lightSVG);
    expect(darkSVG?.dataset.m2hZenumlTheme).toBe("dark");
    expect(darkSVG?.querySelector('[data-upstream="true"]')).not.toBeNull();
    expect(darkStyle?.textContent).toContain(
      'svg[data-m2h-zenuml-theme="dark"] .participant-box',
    );
    expect(darkStyle?.textContent).toContain("fill: #1f2020");
    expect(darkStyle?.textContent).toContain("fill: #cccccc");
  });

  it("registers the plugin once for several ZenUML diagrams", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">zenuml\n    Alice->Bob: Hello</code></pre>' +
      '<pre><code class="language-mermaid">zenuml\n    Bob->Alice: Hi</code></pre>';

    await renderRichContent(root, "light");

    expect(ensureZenUMLRegisteredMock).toHaveBeenCalledTimes(1);
    expect(mermaidMock.render).toHaveBeenCalledTimes(2);
  });

  it("renders ZenUML and flowchart together after one registration", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">zenuml\n    Alice->Bob: Hello</code></pre>' +
      '<pre><code class="language-mermaid">flowchart TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    expect(ensureZenUMLRegisteredMock).toHaveBeenCalledTimes(1);
    expect(mermaidMock.render).toHaveBeenCalledTimes(2);
    expect(root.querySelectorAll("div.mermaid svg")).toHaveLength(2);
  });

  it("re-attempts registration when a theme rerender repaints ZenUML diagrams", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">zenuml\n    Alice->Bob: Hello</code></pre>';

    await renderRichContent(root, "light");
    mermaidMock.render.mockClear();

    await rerenderMermaid(root, "dark");

    // The rerender re-runs preparation (the loader singleton decides whether
    // a second network fetch happens), so a first-attempt failure or a
    // reloaded runtime can still recover on theme switch.
    expect(ensureZenUMLRegisteredMock).toHaveBeenCalledTimes(2);
    expect(mermaidMock.initialize).toHaveBeenCalledWith(
      expect.objectContaining({ theme: "dark" }),
    );
    expect(mermaidMock.render).toHaveBeenCalledTimes(1);
  });

  it("fails the render when plugin registration fails", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    ensureZenUMLRegisteredMock.mockRejectedValueOnce(
      new Error("zenuml plugin unavailable"),
    );
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">zenuml\n    Alice->Bob: Hello</code></pre>';

    // A plugin that cannot be registered is a runtime failure like a failed
    // mermaid.min.js fetch — the document cannot render its diagrams, so the
    // error surfaces instead of silently skipping every zenuml block.
    await expect(renderRichContent(root, "light")).rejects.toThrow(
      "zenuml plugin unavailable",
    );
    expect(mermaidMock.render).not.toHaveBeenCalled();
  });
});

describe("sortable tables", () => {
  beforeEach(() => {
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.registerExternalDiagrams.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    ensureZenUMLRegisteredMock.mockClear();
    loadKatexMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
    });
    installTablesortRuntime();
    loadTablesortMock.mockReset();
    loadTablesortMock.mockImplementation(async () => {
      const Tablesort = window.Tablesort;
      if (Tablesort === undefined) {
        throw new Error("tablesort runtime is not installed");
      }
      return Tablesort;
    });
  });

  afterEach(() => {
    delete window.Tablesort;
  });

  it("makes plain GFM tables sortable and skips classed HTML tables", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `${DEMO_TABLE}
      <table class="custom"><thead><tr><th>x</th></tr></thead>
        <tbody><tr><td>1</td></tr><tr><td>2</td></tr></tbody></table>`;

    await renderRichContent(root, "light");

    const plain = root.querySelectorAll<HTMLTableElement>("table")[0];
    const classed = root.querySelectorAll<HTMLTableElement>("table")[1];
    expect(plain?.dataset.m2hSortable).toBe("true");
    const header = plain?.querySelector<HTMLTableCellElement>("thead th");
    expect(header?.getAttribute("role")).toBe("columnheader");
    expect(header?.getAttribute("aria-sort")).toBe("none");
    expect(header?.tabIndex).toBe(0);
    expect(header?.title).toBe("点击升序排序");

    // A class attribute marks user-authored HTML: no marker, no sortable role.
    expect(classed?.hasAttribute("data-m2h-sortable")).toBe(false);
    expect(classed?.querySelector("thead th")?.getAttribute("role")).toBeNull();
    expect(loadTablesortMock).toHaveBeenCalledTimes(1);
  });

  it("skips tables with a single body row", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      "<table><thead><tr><th>only</th></tr></thead><tbody><tr><td>row</td></tr></tbody></table>";

    await renderRichContent(root, "light");

    // The runtime still loads (the selector cannot see row counts), but a
    // one-row table is never wired up: no marker and no sortable header.
    expect(root.querySelector("table")?.hasAttribute("data-m2h-sortable")).toBe(
      false,
    );
    expect(
      root.querySelector("thead th")?.getAttribute("aria-sort"),
    ).toBeNull();
  });

  it("does not load the tablesort runtime without tables", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<h2>heading</h2><p>paragraph</p>";

    await renderRichContent(root, "light");

    expect(loadTablesortMock).not.toHaveBeenCalled();
  });

  it("starts the tablesort download before awaiting mermaid", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `${DEMO_TABLE}<pre><code class="language-mermaid">graph TD</code></pre>`;

    await renderRichContent(root, "light");

    // Both runtimes are needed, and the loader calls must interleave with the
    // tablesort load kicked off first rather than serialized after mermaid.
    expect(loadTablesortMock).toHaveBeenCalledTimes(1);
    expect(loadMermaidMock).toHaveBeenCalledTimes(1);
    expect(loadTablesortMock.mock.invocationCallOrder[0]).toBeLessThan(
      loadMermaidMock.mock.invocationCallOrder[0],
    );
  });

  it("sorts text columns ascending then descending", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    const header = headerAt(root, 0);
    header.click();
    expect(columnValues(root, 0)).toEqual(["Alpha", "Beta", "Gamma"]);
    header.click();
    expect(columnValues(root, 0)).toEqual(["Gamma", "Beta", "Alpha"]);
  });

  it("sorts numbers numerically rather than lexicographically", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    headerAt(root, 4).click();

    // String order would be 120, 42, 8; numeric order is 8, 42, 120.
    expect(columnValues(root, 4)).toEqual(["8", "42", "120"]);
  });

  it("sorts file sizes by magnitude", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    headerAt(root, 2).click();

    // String order would be "1.5 GB", "12 MB", "850 KB".
    expect(columnValues(root, 2)).toEqual(["850 KB", "12 MB", "1.5 GB"]);
  });

  it("sorts ISO dates within a year", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    headerAt(root, 3).click();

    expect(columnValues(root, 3)).toEqual([
      "2026-07-01",
      "2026-08-01",
      "2026-08-14",
    ]);
  });

  it("sorts dot-separated versions numerically per segment", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    headerAt(root, 1).click();

    // 1.10.0 must outrank 1.2.0 instead of comparing "10" < "2" as text.
    expect(columnValues(root, 1)).toEqual(["1.2.0", "1.10.0", "2.0.0"]);
  });

  it("sorts with Enter and Space key presses", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    const header = headerAt(root, 0);
    header.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter" }));
    expect(columnValues(root, 0)).toEqual(["Alpha", "Beta", "Gamma"]);

    header.dispatchEvent(new KeyboardEvent("keydown", { key: " " }));
    expect(columnValues(root, 0)).toEqual(["Gamma", "Beta", "Alpha"]);

    // Other keys leave the table alone.
    header.dispatchEvent(new KeyboardEvent("keydown", { key: "ArrowDown" }));
    expect(columnValues(root, 0)).toEqual(["Gamma", "Beta", "Alpha"]);
  });

  it("keeps aria-sort and titles in step through the sort cycle", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");

    const project = headerAt(root, 0);
    expect(project.getAttribute("aria-sort")).toBe("none");
    expect(project.title).toBe("点击升序排序");

    project.click();
    expect(project.getAttribute("aria-sort")).toBe("ascending");
    expect(project.title).toBe("当前升序，点击切换为降序");

    project.click();
    expect(project.getAttribute("aria-sort")).toBe("descending");
    expect(project.title).toBe("当前降序，点击切换为升序");

    // Sorting another column restores the explicit "none" baseline — and the
    // default title — on the column that is no longer driving the sort.
    headerAt(root, 1).click();
    expect(project.getAttribute("aria-sort")).toBe("none");
    expect(project.title).toBe("点击升序排序");
    expect(headerAt(root, 1).getAttribute("aria-sort")).toBe("ascending");
  });

  it("excludes headers with interactive content from sorting", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<table><thead><tr><th><a href="#docs">Docs</a></th><th>Version</th></tr></thead>
      <tbody>
        <tr><td>guide</td><td>2.0.0</td></tr>
        <tr><td>api</td><td>1.0.0</td></tr>
      </tbody></table>`;
    await renderRichContent(root, "light");

    const linkHeader = headerAt(root, 0);
    expect(linkHeader.getAttribute("data-sort-method")).toBe("none");
    // No sort affordances: the link keeps its native click meaning.
    expect(linkHeader.getAttribute("aria-sort")).toBeNull();
    expect(linkHeader.title).toBe("");
    expect(linkHeader.tabIndex).toBe(-1);

    linkHeader.click();
    expect(columnValues(root, 1)).toEqual(["2.0.0", "1.0.0"]);

    // The plain column right next to it still sorts.
    headerAt(root, 1).click();
    expect(columnValues(root, 1)).toEqual(["1.0.0", "2.0.0"]);
  });

  it("does not stack a second Tablesort instance on repeated enhancement", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = DEMO_TABLE;
    await renderRichContent(root, "light");
    await renderRichContent(root, "light");

    // One click must apply exactly one ascending sort. If the second pass had
    // registered another click handler, this click would toggle the column
    // straight back to its original order.
    headerAt(root, 1).click();
    expect(columnValues(root, 1)).toEqual(["1.2.0", "1.10.0", "2.0.0"]);
    expect(headerAt(root, 1).getAttribute("aria-sort")).toBe("ascending");
    // The marker also keeps the second pass from re-fetching the runtime.
    expect(loadTablesortMock).toHaveBeenCalledTimes(1);
  });

  it("keeps the sort state when only the theme changes", async () => {
    const { renderRichContent, rerenderMermaid } = await import(
      "./render-rich-content"
    );
    const root = document.createElement("div");
    root.innerHTML = `${DEMO_TABLE}<pre><code class="language-mermaid">graph TD</code></pre>`;
    await renderRichContent(root, "light");

    headerAt(root, 1).click();
    const before = columnValues(root, 1);

    // A theme switch regenerates diagrams only; the table DOM — and its
    // Tablesort instance — must survive untouched.
    await rerenderMermaid(root, "dark");

    expect(columnValues(root, 1)).toEqual(before);
    expect(headerAt(root, 1).getAttribute("aria-sort")).toBe("ascending");
    expect(loadTablesortMock).toHaveBeenCalledTimes(1);
  });
});

const DEMO_TABLE = `<table><thead><tr><th>Project</th><th>Version</th><th>Size</th><th>Updated</th><th>Downloads</th></tr></thead>
  <tbody>
    <tr><td>Alpha</td><td>1.10.0</td><td>12 MB</td><td>2026-08-14</td><td>120</td></tr>
    <tr><td>Beta</td><td>1.2.0</td><td>850 KB</td><td>2026-07-01</td><td>8</td></tr>
    <tr><td>Gamma</td><td>2.0.0</td><td>1.5 GB</td><td>2026-08-01</td><td>42</td></tr>
  </tbody></table>`;

function headerAt(root: HTMLElement, column: number): HTMLTableCellElement {
  const header = root.querySelectorAll<HTMLTableCellElement>(
    "table:not(.custom) thead th",
  )[column];
  if (header === undefined) {
    throw new Error(`no header at column ${column}`);
  }
  return header;
}

function columnValues(root: HTMLElement, column: number): string[] {
  return Array.from(
    root.querySelectorAll<HTMLTableRowElement>("table:not(.custom) tbody tr"),
  ).map((row) => row.cells[column]?.textContent?.trim() ?? "");
}

function replaceProperty(
  target: object,
  property: PropertyKey,
  value: unknown,
): () => void {
  const descriptor = Object.getOwnPropertyDescriptor(target, property);
  Object.defineProperty(target, property, { configurable: true, value });
  return () => {
    if (descriptor === undefined) {
      Reflect.deleteProperty(target, property);
      return;
    }
    Object.defineProperty(target, property, descriptor);
  };
}

describe("Vega-Lite charts", () => {
  const VALID_SPEC = JSON.stringify({
    $schema: "https://vega.github.io/schema/vega-lite/v6.json",
    data: { values: [{ month: "Jan", revenue: 120 }] },
    mark: "bar",
    encoding: {
      x: { field: "month", type: "nominal" },
      y: { field: "revenue", type: "quantitative" },
    },
  });

  let warnSpy: ReturnType<typeof vi.spyOn>;

  beforeEach(() => {
    vi.resetModules();
    loadVegaLiteMock.mockClear();
    vegaEmbedMock.mockClear();
    vegaEmbedMock.mockImplementation(async (element: HTMLElement) => {
      element.innerHTML = '<svg data-mock="vega-lite"></svg>';
      return { view: {}, finalize: vi.fn() };
    });
    warnSpy = vi.spyOn(console, "warn").mockImplementation(() => {});
  });

  afterEach(() => {
    warnSpy.mockRestore();
  });

  it("replaces vega-lite code blocks with an embedded SVG chart", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-vega-lite">${VALID_SPEC}</code></pre>`;

    await renderRichContent(root, "light");

    const container = root.querySelector<HTMLDivElement>("div.m2h-vega-lite");
    expect(container).not.toBeNull();
    expect(container?.innerHTML).toContain('data-mock="vega-lite"');
    expect(root.querySelector("pre")).toBeNull();
    expect(root.querySelector("code.language-vega-lite")).toBeNull();
    // Charts never grow a code frame, gutter, or copy control either.
    expect(root.querySelector(".m2h-code-frame")).toBeNull();

    const frame = root.querySelector(
      ".m2h-rich-visual-frame.m2h-vega-lite-frame",
    );
    expect(frame).not.toBeNull();
    const trigger = frame?.querySelector<HTMLButtonElement>(
      ":scope > .m2h-lightbox-trigger",
    );
    expect(trigger?.hidden).toBe(false);
    expect(container?.dataset.m2hLightboxItem).toBe("true");
    expect(trigger?.getAttribute("aria-label")).toBe("查看 Vega-Lite 图表");
  });

  it("renders the vegalite alias through the same path", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-vegalite">${VALID_SPEC}</code></pre>`;

    await renderRichContent(root, "light");

    expect(root.querySelector("div.m2h-vega-lite svg")).not.toBeNull();
    expect(vegaEmbedMock).toHaveBeenCalledTimes(1);
  });

  it("embeds with pinned host policy options", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-vega-lite">${VALID_SPEC}</code></pre>`;

    await renderRichContent(root, "light");

    expect(vegaEmbedMock).toHaveBeenCalledTimes(1);
    const [, , options] = vegaEmbedMock.mock.calls[0] ?? [];
    expect(options?.mode).toBe("vega-lite");
    expect(options?.renderer).toBe("svg");
    expect(options?.actions).toBe(false);
    expect(options?.tooltip).toBe(true);
    // The deny-network loader rejects external resources instead of relying
    // on a page CSP, so the WebUI and exported HTML share one contract.
    const loader = options?.loader;
    expect(loader).toBeDefined();
    await expect(
      loader?.load("https://example.invalid/data.csv"),
    ).rejects.toThrow("external Vega-Lite data loading is not supported");
    const sanitize = loader?.sanitize;
    expect(sanitize).toBeDefined();
    await expect(sanitize?.("./sales.csv")).rejects.toThrow(
      "external Vega-Lite data loading is not supported",
    );
  });

  it("strips usermeta.embedOptions so the document cannot override host policy", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const spec = JSON.parse(VALID_SPEC) as Record<string, unknown>;
    spec.usermeta = {
      embedOptions: {
        actions: true,
        renderer: "canvas",
        editorUrl: "https://evil.example",
      },
      note: "author data",
    };
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-vega-lite">${JSON.stringify(spec)}</code></pre>`;

    await renderRichContent(root, "light");

    const [element, embedded] = vegaEmbedMock.mock.calls[0] ?? [];
    expect(element.classList.contains("m2h-vega-lite")).toBe(true);
    const embeddedSpec = embedded as Record<string, unknown>;
    const usermeta = embeddedSpec.usermeta as Record<string, unknown>;
    expect("embedOptions" in usermeta).toBe(false);
    expect(usermeta.note).toBe("author data");
  });

  it("isolates an invalid JSON spec and keeps its source text", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-vega-lite">{ not json</code></pre>' +
      `<pre><code class="language-vega-lite">${VALID_SPEC}</code></pre>`;

    await renderRichContent(root, "light");

    const containers =
      root.querySelectorAll<HTMLDivElement>("div.m2h-vega-lite");
    expect(containers).toHaveLength(2);
    // The broken chart keeps its source, gains no SVG, no marker, and a
    // hidden trigger; the valid one after it still renders.
    expect(containers[0]?.textContent).toBe("{ not json");
    expect(containers[0]?.querySelector("svg")).toBeNull();
    expect(containers[0]?.dataset.m2hLightboxItem).toBeUndefined();
    const firstTrigger = containers[0]
      ?.closest(".m2h-vega-lite-frame")
      ?.querySelector<HTMLButtonElement>(":scope > .m2h-lightbox-trigger");
    expect(firstTrigger?.hidden).toBe(true);
    expect(containers[1]?.querySelector("svg")).not.toBeNull();
    expect(vegaEmbedMock).toHaveBeenCalledTimes(1);
    expect(warnSpy).toHaveBeenCalledWith("Failed to render Vega-Lite chart", {
      error: new Error("Vega-Lite specification must be valid JSON"),
    });
  });

  it("rejects a JSON array instead of a spec object", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-vega-lite">[1, 2, 3]</code></pre>';

    await renderRichContent(root, "light");

    expect(vegaEmbedMock).not.toHaveBeenCalled();
    expect(warnSpy).toHaveBeenCalledWith("Failed to render Vega-Lite chart", {
      error: new Error("Vega-Lite specification must be a JSON object"),
    });
    expect(root.querySelector("div.m2h-vega-lite")?.textContent).toBe(
      "[1, 2, 3]",
    );
  });

  it("restores the source and keeps going when the embed rejects", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    vegaEmbedMock.mockRejectedValueOnce(new Error("compile failed"));
    const root = document.createElement("div");
    root.innerHTML = `<pre><code class="language-vega-lite">${VALID_SPEC}</code></pre>`;

    await renderRichContent(root, "light");

    const container = root.querySelector<HTMLDivElement>("div.m2h-vega-lite");
    expect(container?.textContent).toBe(VALID_SPEC);
    expect(container?.dataset.m2hLightboxItem).toBeUndefined();
    expect(warnSpy).toHaveBeenCalledWith("Failed to render Vega-Lite chart", {
      error: new Error("compile failed"),
    });
  });

  it("does not load the Vega-Lite runtime without chart blocks", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<pre><code class="language-go">func main()</code></pre>';

    await renderRichContent(root, "light");

    expect(loadVegaLiteMock).not.toHaveBeenCalled();
  });

  it("does not load the Vega-Lite runtime for Mermaid-only documents", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root, "light");

    expect(loadVegaLiteMock).not.toHaveBeenCalled();
    expect(vegaEmbedMock).not.toHaveBeenCalled();
  });

  it("finalizes and aborts remaining charts when the render goes stale", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const finalize = vi.fn();
    // The document goes away as soon as the first embed settles: the next
    // freshness check reports stale, so that result is finalized and the
    // second chart is never embedded.
    let current = true;
    vegaEmbedMock.mockImplementation(async (element: HTMLElement) => {
      element.innerHTML = '<svg data-mock="vega-lite"></svg>';
      current = false;
      return { view: {}, finalize };
    });
    const root = document.createElement("div");
    root.innerHTML =
      `<pre><code class="language-vega-lite">${VALID_SPEC}</code></pre>` +
      `<pre><code class="language-vega-lite">${VALID_SPEC}</code></pre>`;

    await renderRichContent(root, "light", () => current);

    expect(vegaEmbedMock).toHaveBeenCalledTimes(1);
    expect(finalize).toHaveBeenCalledTimes(1);
    const containers =
      root.querySelectorAll<HTMLDivElement>("div.m2h-vega-lite");
    // The aborted chart keeps its source text instead of an SVG.
    expect(containers[1]?.textContent).toBe(VALID_SPEC);
  });
});
