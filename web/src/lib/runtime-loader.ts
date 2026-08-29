// Loader for the rich-content runtime the Go binary embeds and the document
// server exposes under /runtime/*. The WebUI loads them on demand instead of
// bundling a second copy through Vite; export output loads the same pinned
// releases from the CDN.

export interface MermaidInitializeOptions {
  startOnLoad?: boolean;
  securityLevel?: string;
  theme?: string;
}

export interface MermaidRenderResult {
  svg: string;
  bindFunctions?: (element: HTMLElement) => void;
}

// Shape of a Mermaid external-diagram plugin (see registerExternalDiagrams):
// `detector` decides whether a diagram source belongs to the plugin and
// `loader` produces the diagram implementation Mermaid renders with.
export interface MermaidExternalDiagramDefinition {
  id: string;
  detector: (text: string) => boolean;
  loader: () => Promise<{ id: string; diagram: unknown }>;
}

export interface MermaidRuntime {
  initialize(options: MermaidInitializeOptions): void;
  registerExternalDiagrams(
    diagrams: MermaidExternalDiagramDefinition[],
    options?: {
      lazyLoad?: boolean;
    },
  ): Promise<void>;
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

// The ZenUML plugin keeps its upstream dist layout (see
// internal/assets/rich/NOTICE.md): the entry module lazy-imports its diagram
// chunk through a relative URL, so the chunks directory must stay reachable
// next to this file under /runtime/.
export const ZENUML_MODULE_URL =
  "/runtime/mermaid-zenuml/mermaid-zenuml.esm.min.mjs";

type ZenUMLModuleImporter = () => Promise<{
  default: MermaidExternalDiagramDefinition;
}>;

function importZenUMLModule(): ReturnType<ZenUMLModuleImporter> {
  // The specifier is a runtime URL served by the document server, never a
  // module in Vite's graph; the ignore comment keeps Vite from trying to
  // resolve and bundle it at build time.
  return import(/* @vite-ignore */ ZENUML_MODULE_URL);
}

// One registration per page load: concurrent diagrams share the in-flight
// promise, and a rejection drops the cached promise so the next document (or
// theme rerender) can retry instead of staying broken forever.
let zenumlRegistration: Promise<void> | null = null;

function hostStylesheets(): Set<Element> {
  return new Set(
    Array.from(document.head.children).filter(
      (element) =>
        element instanceof HTMLStyleElement ||
        (element instanceof HTMLLinkElement && element.rel === "stylesheet"),
    ),
  );
}

// External diagram registration is a renderer boundary: importing or
// registering a renderer may return code and SVG markup, but it must never
// install page-wide CSS into m2h's host document. mermaid-zenuml 0.2.3 loads
// @zenuml/core 3.47.2, whose browser bundle appends an unscoped ~888 KiB
// stylesheet during registration. Among its rules are :root --background and
// other generic Tailwind/theme tokens, which overwrite the reader toolbar and
// TOC colors even though the generated native SVG already carries every style
// it needs in its own <defs>. Remove every stylesheet added by this one
// isolated operation on both success and failure; pre-existing app/runtime
// styles are retained by identity.
async function withoutAddedHostStylesheets<T>(
  operation: () => Promise<T>,
): Promise<T> {
  const retained = hostStylesheets();
  try {
    return await operation();
  } finally {
    for (const stylesheet of hostStylesheets()) {
      if (!retained.has(stylesheet)) {
        stylesheet.remove();
      }
    }
  }
}

/**
 * Register the ZenUML external-diagram plugin with the shared Mermaid runtime.
 * Must complete before `mermaid.initialize`, per Mermaid's integration order
 * (load → register → initialize → render). The plugin is fetched only when a
 * document actually contains a `zenuml` diagram; plain Mermaid documents never
 * download it.
 */
export async function ensureZenUMLRegistered(
  mermaid: MermaidRuntime,
  importModule: ZenUMLModuleImporter = importZenUMLModule,
): Promise<void> {
  if (zenumlRegistration !== null) {
    return zenumlRegistration;
  }

  const registration = (async () => {
    await withoutAddedHostStylesheets(async () => {
      const module = await importModule();
      // lazyLoad stays off: the caller already knows this document renders
      // ZenUML, so the diagram chunk downloads during registration and the
      // subsequent render cannot hit a second, hidden lazy load.
      await mermaid.registerExternalDiagrams([module.default], {
        lazyLoad: false,
      });
    });
  })();

  zenumlRegistration = registration;
  try {
    await registration;
  } catch (error) {
    zenumlRegistration = null;
    throw error;
  }
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
