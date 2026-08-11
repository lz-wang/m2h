# Rich-content runtime

Vendored browser assets embedded into the m2h binary so standalone HTML
(`convert` output and single-file `preview`) can render KaTeX math and Mermaid
diagrams offline, without a CDN. The directory WebUI bundles the same libraries
through Vite (see `web/package.json`); this copy serves the standalone path.

| Asset             | Upstream                              | Version | License             |
| ----------------- | ------------------------------------- | ------- | ------------------- |
| KaTeX             | https://github.com/KaTeX/KaTeX        | 0.18.4  | MIT (`LICENSE.katex`)   |
| Mermaid           | https://github.com/mermaid-js/mermaid | 11.16.1 | MIT (`LICENSE.mermaid`) |
| `rich-content.js` | m2h                                   | —       | MIT (project `LICENSE`) |

`katex.min.css` references `./fonts/*.woff2|woff|ttf` relative to itself, so the
`fonts/` directory must stay next to `katex.min.css` in every output location
(the `.m2h/` directory for `convert`, the `/m2h-assets/` route for `preview`).
