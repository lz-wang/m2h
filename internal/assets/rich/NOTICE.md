# Rich-content runtime

Vendored browser assets embedded into the m2h binary so rich content renders
offline, without a CDN. Convert output and the WebUI share this single copy:
standalone HTML embeds it into the page, directory conversion writes it to
`.m2h/`, and the preview server serves it under `/runtime/*`.

| Asset             | Upstream                              | Version | License             |
| ----------------- | ------------------------------------- | ------- | ------------------- |
| KaTeX             | https://github.com/KaTeX/KaTeX        | 0.18.4  | MIT (`LICENSE.katex`)   |
| Mermaid           | https://github.com/mermaid-js/mermaid | 11.16.1 | MIT (`LICENSE.mermaid`) |
| `rich-content.js` | m2h                                   | —       | MIT (project `LICENSE`) |

`katex.min.css` references `./fonts/*.woff2` relative to itself, so the
`fonts/` directory must stay next to `katex.min.css` in every output location.
Only the WOFF2 fonts are vendored — m2h targets modern browsers, and dropping
the woff/ttf fallbacks keeps the embedded runtime and self-contained HTML small.
