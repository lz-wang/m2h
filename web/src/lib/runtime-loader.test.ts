import { waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type {
  MathAutoRenderer,
  MermaidRuntime,
  TablesortConstructor,
} from "./runtime-loader";

const mermaidRuntime: MermaidRuntime = {
  initialize: vi.fn(),
  run: vi.fn(async () => {}),
  render: vi.fn(async () => ({ svg: "<svg></svg>" })),
};

const renderMathInElement: MathAutoRenderer = vi.fn();

const tablesortCtor = vi.fn() as unknown as TablesortConstructor;

const TABLESORT_SCRIPTS = [
  "/runtime/tablesort.min.js",
  "/runtime/tablesort.date.js",
  "/runtime/tablesort.dotsep.js",
  "/runtime/tablesort.filesize.js",
  "/runtime/tablesort.monthname.js",
  "/runtime/tablesort.number.js",
];

function headElement<T extends HTMLElement>(selector: string): T {
  const element = document.head.querySelector<T>(selector);
  if (element === null) {
    throw new Error(`missing injected element ${selector}`);
  }
  return element;
}

async function waitForHeadElement<T extends HTMLElement>(
  selector: string,
): Promise<T> {
  await waitFor(() => {
    headElement<T>(selector);
  });
  return headElement<T>(selector);
}

function fire(element: Element, type: string): void {
  element.dispatchEvent(new Event(type));
}

describe("runtime loader", () => {
  beforeEach(() => {
    // Each test re-imports the module so loader singletons reset.
    vi.resetModules();
    document.head.innerHTML = "";
    delete window.mermaid;
    delete window.renderMathInElement;
    delete window.Tablesort;
    vi.mocked(mermaidRuntime.initialize).mockClear();
  });

  it("injects the mermaid script and resolves with window.mermaid", async () => {
    const pending = import("./runtime-loader").then((loader) =>
      loader.loadMermaid(),
    );
    const script = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/mermaid.min.js"]',
    );

    window.mermaid = mermaidRuntime;
    fire(script, "load");

    await expect(pending).resolves.toBe(mermaidRuntime);
  });

  it("attaches the mermaid runtime without configuring its theme", async () => {
    const loader = await import("./runtime-loader");
    window.mermaid = mermaidRuntime;

    const first = loader.loadMermaid();
    fire(headElement('script[src="/runtime/mermaid.min.js"]'), "load");
    const runtime = await first;
    const second = loader.loadMermaid();
    await second;

    // Theme configuration is the caller's responsibility (render-rich-content),
    // which re-runs initialize whenever the resolved theme changes; the loader
    // only attaches the runtime and never calls initialize itself.
    expect(runtime).toBe(mermaidRuntime);
    expect(mermaidRuntime.initialize).not.toHaveBeenCalled();
  });

  it("deduplicates concurrent mermaid loads to a single script", async () => {
    const loader = await import("./runtime-loader");
    const first = loader.loadMermaid();
    const second = loader.loadMermaid();
    const script = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/mermaid.min.js"]',
    );

    window.mermaid = mermaidRuntime;
    fire(script, "load");

    await Promise.all([first, second]);
    expect(
      document.head.querySelectorAll('script[src="/runtime/mermaid.min.js"]'),
    ).toHaveLength(1);
  });

  it("rejects when the mermaid script fails and allows a retry", async () => {
    const loader = await import("./runtime-loader");
    const first = loader.loadMermaid();
    const script = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/mermaid.min.js"]',
    );

    fire(script, "error");
    await expect(first).rejects.toThrow(
      "load runtime script /runtime/mermaid.min.js",
    );
    expect(
      document.head.querySelector('script[src="/runtime/mermaid.min.js"]'),
    ).toBeNull();

    const retry = loader.loadMermaid();
    const retriedScript = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/mermaid.min.js"]',
    );
    window.mermaid = mermaidRuntime;
    fire(retriedScript, "load");
    await expect(retry).resolves.toBe(mermaidRuntime);
  });

  it("rejects when the loaded script does not attach window.mermaid", async () => {
    const loader = await import("./runtime-loader");
    const pending = loader.loadMermaid();
    const script = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/mermaid.min.js"]',
    );

    fire(script, "load");

    await expect(pending).rejects.toThrow(
      "mermaid runtime did not attach window.mermaid",
    );
  });

  it("loads the KaTeX stylesheet, core, and auto-render in order", async () => {
    const loader = await import("./runtime-loader");
    const pending = loader.loadKatex();
    const stylesheet = await waitForHeadElement<HTMLLinkElement>(
      'link[rel="stylesheet"][href="/runtime/katex.min.css"]',
    );
    const core = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/katex.min.js"]',
    );

    // auto-render only appears after the stylesheet and core resolve.
    expect(
      document.head.querySelector('script[src="/runtime/auto-render.min.js"]'),
    ).toBeNull();

    fire(stylesheet, "load");
    fire(core, "load");

    const autoRender = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/auto-render.min.js"]',
    );
    window.renderMathInElement = renderMathInElement;
    fire(autoRender, "load");

    await expect(pending).resolves.toBe(renderMathInElement);
  });

  it("rejects a failed KaTeX stylesheet and removes the link", async () => {
    const loader = await import("./runtime-loader");
    const pending = loader.loadKatex();
    const stylesheet = await waitForHeadElement<HTMLLinkElement>(
      'link[rel="stylesheet"][href="/runtime/katex.min.css"]',
    );

    fire(stylesheet, "error");

    await expect(pending).rejects.toThrow(
      "load runtime stylesheet /runtime/katex.min.css",
    );
    expect(
      document.head.querySelector('link[href="/runtime/katex.min.css"]'),
    ).toBeNull();
  });

  it("loads the tablesort core before its comparator extensions", async () => {
    const loader = await import("./runtime-loader");
    const pending = loader.loadTablesort();
    const core = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/tablesort.min.js"]',
    );

    // Extensions only appear after the core script resolves.
    expect(
      document.head.querySelector('script[src="/runtime/tablesort.number.js"]'),
    ).toBeNull();

    window.Tablesort = tablesortCtor;
    fire(core, "load");

    for (const src of TABLESORT_SCRIPTS.slice(1)) {
      const extension = await waitForHeadElement<HTMLScriptElement>(
        `script[src="${src}"]`,
      );
      fire(extension, "load");
    }

    await expect(pending).resolves.toBe(tablesortCtor);
  });

  it("deduplicates concurrent tablesort loads to a single script set", async () => {
    const loader = await import("./runtime-loader");
    window.Tablesort = tablesortCtor;
    const first = loader.loadTablesort();
    const second = loader.loadTablesort();
    const core = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/tablesort.min.js"]',
    );
    fire(core, "load");
    for (const src of TABLESORT_SCRIPTS.slice(1)) {
      const extension = await waitForHeadElement<HTMLScriptElement>(
        `script[src="${src}"]`,
      );
      fire(extension, "load");
    }

    await Promise.all([first, second]);
    for (const src of TABLESORT_SCRIPTS) {
      expect(
        document.head.querySelectorAll(`script[src="${src}"]`),
      ).toHaveLength(1);
    }
  });

  it("rejects when the core does not attach window.Tablesort", async () => {
    const loader = await import("./runtime-loader");
    const pending = loader.loadTablesort();
    const core = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/tablesort.min.js"]',
    );

    // Every script reports a successful load, yet the global never appears.
    fire(core, "load");
    for (const src of TABLESORT_SCRIPTS.slice(1)) {
      const extension = await waitForHeadElement<HTMLScriptElement>(
        `script[src="${src}"]`,
      );
      fire(extension, "load");
    }

    await expect(pending).rejects.toThrow(
      "Tablesort runtime did not attach window.Tablesort",
    );
  });

  it("rejects a failed tablesort load and allows a retry", async () => {
    const loader = await import("./runtime-loader");
    const first = loader.loadTablesort();
    const core = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/tablesort.min.js"]',
    );

    fire(core, "error");
    await expect(first).rejects.toThrow(
      "load runtime script /runtime/tablesort.min.js",
    );
    expect(
      document.head.querySelector('script[src="/runtime/tablesort.min.js"]'),
    ).toBeNull();

    const retry = loader.loadTablesort();
    const retriedCore = await waitForHeadElement<HTMLScriptElement>(
      'script[src="/runtime/tablesort.min.js"]',
    );
    window.Tablesort = tablesortCtor;
    fire(retriedCore, "load");
    for (const src of TABLESORT_SCRIPTS.slice(1)) {
      const extension = await waitForHeadElement<HTMLScriptElement>(
        `script[src="${src}"]`,
      );
      fire(extension, "load");
    }

    await expect(retry).resolves.toBe(tablesortCtor);
  });
});
