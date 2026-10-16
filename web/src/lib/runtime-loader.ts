// Loader for the rich-content runtime the Go binary embeds and the document
// server exposes under /runtime/*. The WebUI loads them on demand instead of
// bundling a second copy through Vite. With --cdn the server's shell opts into
// the same pinned CDN releases used by export output.

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
  render(id: string, text: string): Promise<MermaidRenderResult>;
}

export interface MathAutoRenderDelimiter {
  left: string;
  right: string;
  display?: boolean;
}

export interface MathAutoRenderOptions {
  delimiters?: MathAutoRenderDelimiter[];
  ignoredClasses?: string[];
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

// Keep versions in sync with internal/assets/rich/NOTICE.md and export/page.go.
// Capture only the server-owned head metadata, before document HTML is mounted.
const useCDN =
  document.head.querySelector<HTMLMetaElement>('meta[name="m2h-cdn"]')
    ?.content === "true";
const CDN_BASE = "https://cdn.jsdelivr.net/npm/";
const CDN_PATHS = {
  "mermaid.min.js": "mermaid@12.2.0/dist/mermaid.min.js",
  "katex.min.css": "katex@0.18.4/dist/katex.min.css",
  "katex.min.js": "katex@0.18.4/dist/katex.min.js",
  "auto-render.min.js": "katex@0.18.4/dist/contrib/auto-render.min.js",
  "tablesort.min.js": "tablesort@5.3.0/dist/tablesort.min.js",
  "tablesort.date.js": "tablesort@5.3.0/dist/sorts/tablesort.date.min.js",
  "tablesort.dotsep.js": "tablesort@5.3.0/dist/sorts/tablesort.dotsep.min.js",
  "tablesort.filesize.js":
    "tablesort@5.3.0/dist/sorts/tablesort.filesize.min.js",
  "tablesort.monthname.js":
    "tablesort@5.3.0/dist/sorts/tablesort.monthname.min.js",
  "tablesort.number.js": "tablesort@5.3.0/dist/sorts/tablesort.number.min.js",
} as const;

function runtimeURL(asset: keyof typeof CDN_PATHS): string {
  return useCDN ? `${CDN_BASE}${CDN_PATHS[asset]}` : `${RUNTIME_BASE}${asset}`;
}

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
  await injectScript(runtimeURL("mermaid.min.js"));
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
    injectStyle(runtimeURL("katex.min.css")),
    injectScript(runtimeURL("katex.min.js")),
  ]);
  await injectScript(runtimeURL("auto-render.min.js"));
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
  await injectScript(runtimeURL("tablesort.min.js"));

  await Promise.all([
    injectScript(runtimeURL("tablesort.date.js")),
    injectScript(runtimeURL("tablesort.dotsep.js")),
    injectScript(runtimeURL("tablesort.filesize.js")),
    injectScript(runtimeURL("tablesort.monthname.js")),
    injectScript(runtimeURL("tablesort.number.js")),
  ]);

  const runtime = window.Tablesort;
  if (runtime === undefined) {
    throw new Error("Tablesort runtime did not attach window.Tablesort");
  }
  return runtime;
}
