# Rich-content runtime

Vendored browser assets embedded into the m2h binary so the WebUI renders rich
content offline, without a CDN. The document server serves them under
`/runtime/*`; the WebUI loads them on demand instead of bundling a second copy
through Vite. Exported HTML (m2h export) does not use this copy — it loads
the same pinned releases from jsDelivr, except that it pulls only the
Tablesort core: the five comparator builds below stay WebUI-only.

| Asset             | Upstream                              | Version | License             |
| ----------------- | ------------------------------------- | ------- | ------------------- |
| KaTeX             | https://github.com/KaTeX/KaTeX        | 0.18.4  | MIT (`LICENSE.katex`)   |
| Mermaid           | https://github.com/mermaid-js/mermaid | 11.16.1 | MIT (`LICENSE.mermaid`) |
| mermaid-zenuml    | https://github.com/mermaid-js/mermaid (packages/mermaid-zenuml) | 0.2.3 | MIT (`LICENSE.mermaid-zenuml`) |
| @zenuml/core      | https://github.com/mermaid-js/zenuml (bundled inside mermaid-zenuml) | 3.47.2 | MIT (`LICENSE.zenuml-core`) |
| Tablesort         | https://github.com/tristen/tablesort  | 5.3.0   | MIT (`LICENSE.tablesort`) |

`katex.min.css` references `./fonts/*.woff2` relative to itself, so the
`fonts/` directory must stay next to `katex.min.css` in every output location.
Only the WOFF2 fonts are vendored — m2h targets modern browsers, and dropping
the woff/ttf fallbacks keeps the embedded runtime small.

Tablesort keeps its core (`tablesort.min.js`) and the five vendored sort
comparators (`tablesort.number|date|filesize|dotsep|monthname.js`) flat in this
directory; the comparators are the upstream minified `dist/sorts` builds renamed
without the `.min` infix. Note the 5.3.0 npm package ships dist builds whose
banner still reads v5.2.1 — upstream published the release without rebuilding
dist — so the npm version, not the banner, is authoritative here.

ZenUML lives in `mermaid-zenuml/` and keeps the upstream `dist/` file names and
relative paths (`mermaid-zenuml.esm.min.mjs` plus
`chunks/mermaid-zenuml.esm.min/*`), because the entry module lazy-imports its
diagram chunk through a relative URL at registration time — renaming or
flattening the chunks would break that import. It is the browser-side,
self-contained build: the npm `module` entry (`mermaid-zenuml.core.mjs`) keeps
`@zenuml/core` as a bare bundler external, which no browser can resolve, while
`mermaid-zenuml.esm.min.mjs` bundles the ZenUML engine and exports the same
`{ id, detector, loader }` external-diagram plugin. Source maps are not
vendored. The WebUI fetches this directory only when a document actually
contains a `zenuml` diagram; Mermaid Core (`mermaid.min.js`) alone handles every
other diagram type. Exported HTML does not embed this copy — the page carries
the pinned jsDelivr URL of the same release, and its bootstrap downloads the
plugin on demand under the same leading-keyword rule.

The 0.2.3 browser build bundles @zenuml/core 3.47.2 exactly. Its registration
path injects the core's application stylesheet into the host `<head>`, even
though the native SVG renderer already returns self-styled SVG markup. Both
the WebUI loader and exported-page bootstrap therefore treat registration as a
no-host-stylesheet boundary: stylesheets added during import/registration are
removed on success or failure before Mermaid renders the returned SVG.

The core's static SVG output is light-only in this pinned release. m2h leaves
that upstream palette unchanged in light mode and appends a constant, SVG-root
scoped dark palette when the resolved reader mode is dark. The scoped rules are
part of the SVG itself (including Lightbox snapshots and exported HTML) and do
not alter host stylesheets or theme variables.
