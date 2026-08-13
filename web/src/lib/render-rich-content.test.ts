import { waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  MathAutoRenderer,
  MathAutoRenderOptions,
  MermaidRenderResult,
  MermaidRuntime,
} from "./runtime-loader";

interface MermaidRunOptions {
  nodes: HTMLElement[];
  suppressErrors: boolean;
}

const mermaidMock = vi.hoisted(() => ({
  initialize: vi.fn<(config: unknown) => void>(),
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

const loadKatexMock = vi.hoisted(() =>
  vi.fn(async (): Promise<MathAutoRenderer> => renderMathInElementMock),
);

vi.mock("./runtime-loader", () => ({
  loadMermaid: loadMermaidMock,
  loadKatex: loadKatexMock,
}));

describe("renderRichContent", () => {
  beforeEach(() => {
    // Each test re-imports the module so the lazy mermaid singleton resets.
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    loadKatexMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
    mermaidMock.render.mockResolvedValue({
      svg: '<svg data-mock="mermaid"></svg>',
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

    const lefts =
      options.delimiters?.map(
        (delimiter: { left: string }) => delimiter.left,
      ) ?? [];
    expect(lefts.indexOf("$$")).toBeLessThan(lefts.indexOf("$"));
    expect(lefts).toEqual(["$$", "\\[", "\\(", "$"]);
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

describe("rerenderMermaid", () => {
  beforeEach(() => {
    // Each test re-imports the module so the lazy mermaid singleton and the
    // mermaidSources WeakMap reset between cases.
    vi.resetModules();
    mermaidMock.initialize.mockClear();
    mermaidMock.run.mockClear();
    mermaidMock.render.mockClear();
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    loadKatexMock.mockClear();
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
