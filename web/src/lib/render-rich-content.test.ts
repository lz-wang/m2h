import { waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  MathAutoRenderer,
  MathAutoRenderOptions,
  MermaidRuntime,
} from "./runtime-loader";

interface MermaidRunOptions {
  nodes: HTMLElement[];
  suppressErrors: boolean;
}

const mermaidMock = vi.hoisted(() => ({
  initialize: vi.fn<(config: unknown) => void>(),
  run: vi.fn<(options: MermaidRunOptions) => Promise<void>>(async () => {}),
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
    renderMathInElementMock.mockClear();
    loadMermaidMock.mockClear();
    loadKatexMock.mockClear();
    mermaidMock.run.mockResolvedValue(undefined);
  });

  it("replaces mermaid code blocks with diagram containers and runs mermaid", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root);

    const container = root.querySelector("div.mermaid");
    expect(container).not.toBeNull();
    expect(container?.textContent).toContain("graph TD");
    expect(root.querySelector("pre")).toBeNull();
    expect(root.querySelector("code.language-mermaid")).toBeNull();

    expect(mermaidMock.run).toHaveBeenCalledTimes(1);
    const options = mermaidMock.run.mock.calls[0]?.[0];
    expect(options).toBeDefined();
    expect(options?.nodes).toHaveLength(1);
    expect(options?.suppressErrors).toBe(true);
  });

  it("leaves non-mermaid code blocks untouched and skips mermaid.run", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = '<pre><code class="language-go">func main()</code></pre>';

    await renderRichContent(root);

    expect(root.querySelector("pre")).not.toBeNull();
    expect(root.querySelector("div.mermaid")).toBeNull();
    expect(mermaidMock.run).not.toHaveBeenCalled();
    expect(
      root.querySelector<HTMLButtonElement>(".m2h-code-copy"),
    ).not.toBeNull();
  });

  it("does not load the mermaid runtime without diagram blocks", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<p>plain text and $math$ only</p>";

    await renderRichContent(root);

    expect(loadMermaidMock).not.toHaveBeenCalled();
    expect(loadKatexMock).toHaveBeenCalledTimes(1);
  });

  it("does not load the KaTeX runtime without math delimiters", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre>';

    await renderRichContent(root);

    expect(loadMermaidMock).toHaveBeenCalledTimes(1);
    expect(loadKatexMock).not.toHaveBeenCalled();
  });

  it("loads no runtime for a plain document", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML = "<h2>heading</h2><p>paragraph without delimiters</p>";

    await renderRichContent(root);

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

      await renderRichContent(root);

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

      await renderRichContent(root);

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

    await renderRichContent(root);

    expect(mermaidMock.run).not.toHaveBeenCalled();
    expect(root.querySelector("code.language-mermaid")).not.toBeNull();
  });

  it("hands the root to KaTeX with $$ before $ and throwOnError disabled", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.textContent = "$E=mc^2$";

    await renderRichContent(root);

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
    mermaidMock.run.mockImplementation(async () => {
      order.push("mermaid");
    });
    renderMathInElementMock.mockImplementation(() => {
      order.push("katex");
    });

    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>$E=mc^2$</p>';

    await renderRichContent(root);

    expect(order).toEqual(["mermaid", "katex"]);
  });

  it("skips KaTeX when a stale render is reported after mermaid resolves", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>$E=mc^2$</p>';

    // Mermaid runs (the mock resolves), but the render is no longer current, so
    // KaTeX must not scan the now-stale root.
    await renderRichContent(root, () => false);

    expect(mermaidMock.run).toHaveBeenCalledTimes(1);
    expect(renderMathInElementMock).not.toHaveBeenCalled();
  });

  it("still renders KaTeX when the freshness check reports current", async () => {
    const { renderRichContent } = await import("./render-rich-content");
    const root = document.createElement("div");
    root.innerHTML =
      '<pre><code class="language-mermaid">graph TD\nA--&gt;B</code></pre><p>$E=mc^2$</p>';

    await renderRichContent(root, () => true);

    expect(renderMathInElementMock).toHaveBeenCalledTimes(1);
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
