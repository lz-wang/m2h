// Loader for the rich-content runtime the Go binary embeds and the preview
// server exposes under /runtime/*. Sharing those assets with convert output
// keeps a single Mermaid/KaTeX runtime in the binary; the WebUI loads them on
// demand instead of bundling a second copy through Vite.

export interface MermaidInitializeOptions {
  startOnLoad?: boolean;
  securityLevel?: string;
  theme?: string;
}

export interface MermaidRenderResult {
  svg: string;
  bindFunctions?: (element: HTMLElement) => void;
}

export interface MermaidRuntime {
  initialize(options: MermaidInitializeOptions): void;
  run(options: {
    nodes: HTMLElement[];
    suppressErrors: boolean;
  }): Promise<void>;
  render(id: string, text: string): Promise<MermaidRenderResult>;
}

export interface MathAutoRenderDelimiter {
  left: string;
  right: string;
  display?: boolean;
}

export interface MathAutoRenderOptions {
  delimiters?: MathAutoRenderDelimiter[];
  throwOnError?: boolean;
}

export type MathAutoRenderer = (
  element: HTMLElement,
  options?: MathAutoRenderOptions,
) => void;

export interface TablesortInstance {
  refresh(): void;
}

export interface TablesortConstructor {
  new (
    table: HTMLTableElement,
    options?: {
      descending?: boolean;
      sortAttribute?: string;
    },
  ): TablesortInstance;
}

declare global {
  interface Window {
    mermaid?: MermaidRuntime;
    renderMathInElement?: MathAutoRenderer;
    Tablesort?: TablesortConstructor;
  }
}

const RUNTIME_BASE = "/runtime/";

const scriptLoads = new Map<string, Promise<void>>();
const styleLoads = new Map<string, Promise<void>>();

function injectScript(src: string): Promise<void> {
  const pending = scriptLoads.get(src);
  if (pending !== undefined) {
    return pending;
  }
  const load = new Promise<void>((resolve, reject) => {
    const script = document.createElement("script");
    script.src = src;
    script.addEventListener("load", () => {
      resolve();
    });
    script.addEventListener("error", () => {
      // Drop the cached rejection so a later call can retry the load.
      scriptLoads.delete(src);
      script.remove();
      reject(new Error(`load runtime script ${src}`));
    });
    document.head.append(script);
  });
  scriptLoads.set(src, load);
  return load;
}

function injectStyle(href: string): Promise<void> {
  const pending = styleLoads.get(href);
  if (pending !== undefined) {
    return pending;
  }
  const load = new Promise<void>((resolve, reject) => {
    const link = document.createElement("link");
    link.rel = "stylesheet";
    link.href = href;
    link.addEventListener("load", () => {
      resolve();
    });
    link.addEventListener("error", () => {
      styleLoads.delete(href);
      link.remove();
      reject(new Error(`load runtime stylesheet ${href}`));
    });
    document.head.append(link);
  });
  styleLoads.set(href, load);
  return load;
}

/**
 * Load the shared Mermaid runtime. The loader only attaches the script and
 * resolves with `window.mermaid`; it deliberately does not call `initialize`,
 * because the theme must be chosen (and re-chosen on theme change) by the
 * caller that knows the resolved light/dark mode. Rejects when the script
 * cannot be fetched or does not attach `window.mermaid`.
 */
export async function loadMermaid(): Promise<MermaidRuntime> {
  await injectScript(`${RUNTIME_BASE}mermaid.min.js`);
  const runtime = window.mermaid;
  if (runtime === undefined) {
    throw new Error("mermaid runtime did not attach window.mermaid");
  }
  return runtime;
}

/**
 * Load the shared KaTeX runtime — stylesheet, core, then the auto-render
 * extension — and return `renderMathInElement`.
 */
export async function loadKatex(): Promise<MathAutoRenderer> {
  await Promise.all([
    injectStyle(`${RUNTIME_BASE}katex.min.css`),
    injectScript(`${RUNTIME_BASE}katex.min.js`),
  ]);
  await injectScript(`${RUNTIME_BASE}auto-render.min.js`);
  const renderMath = window.renderMathInElement;
  if (renderMath === undefined) {
    throw new Error("KaTeX runtime did not attach renderMathInElement");
  }
  return renderMath;
}

/**
 * Load the shared Tablesort runtime — the core first (it defines
 * `window.Tablesort`), then every vendored comparator extension. The
 * extensions only call `Tablesort.extend` to register comparators and never
 * depend on each other, so they load concurrently. Rejects when the scripts
 * cannot be fetched or the core does not attach `window.Tablesort`.
 */
export async function loadTablesort(): Promise<TablesortConstructor> {
  await injectScript(`${RUNTIME_BASE}tablesort.min.js`);

  await Promise.all([
    injectScript(`${RUNTIME_BASE}tablesort.date.js`),
    injectScript(`${RUNTIME_BASE}tablesort.dotsep.js`),
    injectScript(`${RUNTIME_BASE}tablesort.filesize.js`),
    injectScript(`${RUNTIME_BASE}tablesort.monthname.js`),
    injectScript(`${RUNTIME_BASE}tablesort.number.js`),
  ]);

  const runtime = window.Tablesort;
  if (runtime === undefined) {
    throw new Error("Tablesort runtime did not attach window.Tablesort");
  }
  return runtime;
}
