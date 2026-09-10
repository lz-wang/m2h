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

// Host-controlled embed options, narrowed to the fields m2h sets. The embed
// call always passes an explicit options object — renderer policy belongs to
// the host application, never to the Markdown document — so mode, renderer,
// and actions are required rather than optional here.
export interface VegaEmbedOptions {
  // Always "vega-lite": without it Vega-Embed infers the mode from $schema
  // and falls back to raw Vega when both are absent.
  mode: "vega-lite";
  // SVG keeps charts in the same vector pipeline as Mermaid: theme-aware,
  // Lightbox-serializable, and crisp when printed or zoomed.
  renderer: "svg";
  // Vega-Embed's own Export/Source/Editor menu would both duplicate m2h's UI
  // and navigate away to the external Vega Editor.
  actions: false;
  // Evaluate chart expressions through the bundled AST interpreter instead
  // of Vega's generated-code path (new Function). This is what keeps charts
  // working under the page CSP's script-src 'self' — no 'unsafe-eval' — at
  // a modest parse/dataflow cost this documentation viewer can afford.
  ast: true;
  config?: VegaLiteHostConfig;
  loader?: VegaLoader;
  tooltip?: boolean;
}

// The reader palette overlaid onto every spec: chrome colors only —
// background, axis/legend/title text and strokes. The author's mark colors
// and scale ranges are never touched, so a spec's data semantics survive a
// theme switch unchanged. Values come from the reader theme's CSS variables
// (computed at call time), keeping the stylesheet the single source of truth.
export interface VegaLiteHostConfig {
  background: null;
  axis: {
    labelColor: string;
    titleColor: string;
    gridColor: string;
    domainColor: string;
    tickColor: string;
  };
  legend: {
    labelColor: string;
    titleColor: string;
  };
  title: {
    color: string;
  };
  view: {
    stroke: null;
  };
}

// Vega's dataflow loader: `load` fetches a URI after `sanitize` vetted it.
// m2h passes a host implementation that rejects every external fetch (data,
// config, patch, images), so specs stay self-contained (data.values only) in
// the WebUI and exported HTML alike — the same contract with or without a
// page CSP. The one exception is the "href" context: Vega sanitizes a chart
// hyperlink click through the loader too, and the result's keys become
// attributes on the anchor it synthesizes, so target/rel there is how the
// host applies its navigation policy.
export interface VegaLoaderRequestOptions {
  context?: string;
}

export interface VegaLoaderSanitized {
  href: string;
  target?: string;
  rel?: string;
}

export interface VegaLoader {
  sanitize?(
    uri: string,
    options?: VegaLoaderRequestOptions,
  ): Promise<VegaLoaderSanitized | string>;
  load(uri: string, options?: VegaLoaderRequestOptions): Promise<string>;
}

// What Vega-Embed resolves with. The compiled Vega View is opaque to m2h —
// only `finalize` matters: it detaches the view's timers and DOM listeners
// and must run whenever a chart is re-rendered or its document goes away.
export interface VegaEmbedResult {
  view: unknown;
  finalize(): void;
}

export type VegaEmbedRuntime = (
  element: HTMLElement,
  spec: object,
  options: VegaEmbedOptions,
) => Promise<VegaEmbedResult>;

declare global {
  interface Window {
    mermaid?: MermaidRuntime;
    renderMathInElement?: MathAutoRenderer;
    Tablesort?: TablesortConstructor;
    vegaEmbed?: VegaEmbedRuntime;
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
  "mermaid.min.js": "mermaid@11.16.1/dist/mermaid.min.js",
  "mermaid-zenuml/mermaid-zenuml.esm.min.mjs":
    "@mermaid-js/mermaid-zenuml@0.2.3/dist/mermaid-zenuml.esm.min.mjs",
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
  "vega.min.js": "vega@6.4.0/build/vega.min.js",
  "vega-lite.min.js": "vega-lite@6.4.3/build/vega-lite.min.js",
  "vega-embed.min.js": "vega-embed@7.1.0/build/vega-embed.min.js",
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

// The ZenUML plugin keeps its upstream dist layout (see
// internal/assets/rich/NOTICE.md): the entry module lazy-imports its diagram
// chunk through a relative URL, so the chunks directory must stay reachable
// next to this file under /runtime/.
export const ZENUML_MODULE_URL = runtimeURL(
  "mermaid-zenuml/mermaid-zenuml.esm.min.mjs",
);

type ZenUMLModuleImporter = () => Promise<{
  default: MermaidExternalDiagramDefinition;
}>;

function importZenUMLModule(): ReturnType<ZenUMLModuleImporter> {
  // The specifier is a runtime URL served locally or by the CDN, never a
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

/**
 * Load the shared Vega-Lite runtime trio and resolve with
 * `window.vegaEmbed`. The three scripts form a dependency chain — vega
 * attaches `window.vega`, vega-lite compiles against that runtime,
 * vega-embed receives both as globals — so they load strictly in sequence,
 * never concurrently. Like the other loaders, this one only attaches the
 * scripts: embed options (mode, renderer, actions, loader) are chosen by the
 * caller on every embed. `injectScript` already caches in-flight loads and
 * drops failed ones for retry, so no second bookkeeping is needed. Rejects
 * when a script cannot be fetched or the chain does not attach
 * `window.vegaEmbed`.
 */
export async function loadVegaLite(): Promise<VegaEmbedRuntime> {
  await injectScript(runtimeURL("vega.min.js"));
  await injectScript(runtimeURL("vega-lite.min.js"));
  await injectScript(runtimeURL("vega-embed.min.js"));

  const runtime = window.vegaEmbed;
  if (runtime === undefined) {
    throw new Error("Vega Embed runtime did not attach window.vegaEmbed");
  }
  return runtime;
}
