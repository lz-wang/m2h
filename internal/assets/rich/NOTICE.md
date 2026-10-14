# Rich-content runtime

Vendored browser assets embedded into the m2h binary so the WebUI renders rich
content offline by default, without a CDN. With `--cdn`, the WebUI instead
loads the same pinned releases directly from jsDelivr, including KaTeX fonts
and all five Tablesort comparators. Its URL mapping lives in
`web/src/lib/runtime-loader.ts` and must track version updates here and in
`internal/export/page.go`. The document server serves embedded assets under
`/runtime/*`; the WebUI loads them on demand instead of bundling a second copy
through Vite. Exported HTML (m2h export) does not use this copy — it loads
the same pinned releases from jsDelivr, except that it pulls only the
Tablesort core: the five comparator builds below stay WebUI-only.

| Asset             | Upstream                              | Version | License             |
| ----------------- | ------------------------------------- | ------- | ------------------- |
| KaTeX             | https://github.com/KaTeX/KaTeX        | 0.18.4  | MIT (`LICENSE.katex`)   |
| Mermaid           | https://github.com/mermaid-js/mermaid | 11.16.1 | MIT (`LICENSE.mermaid`) |
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
