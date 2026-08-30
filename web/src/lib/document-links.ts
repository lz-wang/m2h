// Document navigation policy for the Markdown body.
//
// Cross-origin HTTP(S) links in a rendered document open in a new browsing
// context (target="_blank" plus rel="noopener"); everything else — site URLs,
// fragments, mailto:/tel: — keeps the browser's default behavior. This runs in
// the browser, not the Go renderer, on purpose: behind a reverse proxy the
// server only ever sees its upstream address (e.g. 127.0.0.1:9527) and would
// need proxy-header heuristics to tell internal from external links, while
// window.location.origin already is the public origin the reader browses.
// Setting the DOM attributes (rather than window.open() in a click handler)
// leaves Ctrl/Cmd+click, middle-click and "open in new tab" to the browser.

// Mark every cross-origin HTTP(S) link under `root` to open in a new browsing
// context. `baseHref` (the reader's current URL) exists so callers and tests
// can pin the origin to compare against; the comparison is strictly on origin
// — scheme and port included — because https://host and http://host:8443 are
// separate web origins even when the hostname matches.
export function enhanceDocumentLinks(
  root: HTMLElement,
  baseHref = window.location.href,
): void {
  const current = new URL(baseHref);

  for (const anchor of root.querySelectorAll<HTMLAnchorElement>("a[href]")) {
    const href = anchor.getAttribute("href");
    if (href === null || href === "") {
      continue;
    }

    let target: URL;
    try {
      target = new URL(href, current);
    } catch {
      continue;
    }

    if (target.protocol !== "http:" && target.protocol !== "https:") {
      continue;
    }

    if (target.origin === current.origin) {
      continue;
    }

    anchor.target = "_blank";

    // Merge, never replace: an author's own tokens (e.g. nofollow) survive.
    const rel = new Set(
      (anchor.getAttribute("rel") ?? "").split(/\s+/).filter(Boolean),
    );
    rel.add("noopener");
    anchor.setAttribute("rel", [...rel].join(" "));
  }
}
