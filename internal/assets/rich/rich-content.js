// m2h rich-content runtime for standalone HTML (convert + single-file preview).
//
// Mirrors web/src/lib/render-rich-content.ts: after the browser receives the
// rendered Markdown body, replace ```mermaid fenced blocks with diagram
// containers, let KaTeX scan the remaining text for math, and make plain GFM
// tables sortable. Mermaid runs first so KaTeX never sees raw diagram source,
// and tables sort before the deep-link restore so header padding cannot shift
// the final scroll position. Loaded as a plain <script> after katex.min.js,
// auto-render.min.js, mermaid.min.js and — when the document has tables — the
// tablesort runtime, which attach the `renderMathInElement`, `mermaid` and
// `Tablesort` globals.
(function () {
  "use strict";

  var DELIMITERS = [
    { left: "$$", right: "$$", display: true },
    { left: "\\[", right: "\\]", display: true },
    { left: "\\(", right: "\\)", display: false },
    { left: "$", right: "$", display: false },
  ];
  var COPY_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><rect x="5.25" y="5.25" width="7.25" height="7.25" rx="1.25"></rect><path d="M10.75 5.25V3.5c0-.69-.56-1.25-1.25-1.25H3.5c-.69 0-1.25.56-1.25 1.25v6c0 .69.56 1.25 1.25 1.25h1.75"></path></svg>';
  var COPIED_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m3.5 8.25 2.1 2.1 4.9-4.9"></path></svg>';
  var COPY_FAILED_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m4.5 4.5 7 7m0-7-7 7"></path></svg>';
  var HEADING_ANCHOR_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path fill-rule="evenodd" d="M4 9h1v1H4c-1.5 0-3-1.69-3-3.5S2.55 3 4 3h4c1.45 0 3 1.69 3 3.5 0 1.41-.91 2.72-2 3.25V8.59c.58-.45 1-1.27 1-2.09C10 5.22 8.98 4 8 4H4c-.98 0-2 1.22-2 2.5S3 9 4 9zm9-3h-1v1h1c1 0 2 1.22 2 2.5S13.98 12 13 12H9c-.98 0-2-1.22-2-2.5 0-.83.42-1.64 1-2.09V6.25c-1.09.53-2 1.84-2 3.25C6 11.31 7.55 13 9 13h4c1.45 0 3-1.69 3-3.5S14.5 6 13 6z"></path></svg>';

  function documentRoot() {
    return document.querySelector(".markdown-body");
  }

  // The Go template stamps the chosen mode onto <html> as m2h-mode-light/dark/auto.
  // Resolve it to a concrete light/dark so Mermaid picks the matching official
  // theme; "auto" falls back to prefers-color-scheme at load time.
  function isDarkMode() {
    var root = document.documentElement;
    if (root.classList.contains("m2h-mode-dark")) {
      return true;
    }
    if (root.classList.contains("m2h-mode-light")) {
      return false;
    }
    return (
      typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches
    );
  }

  function renderMath(root) {
    if (typeof renderMathInElement !== "function") {
      return;
    }
    renderMathInElement(root, {
      delimiters: DELIMITERS,
      throwOnError: false,
    });
  }

  function replaceMermaidBlocks(root) {
    var nodes = [];
    root.querySelectorAll("pre > code.language-mermaid").forEach(function (code) {
      var pre = code.parentElement;
      if (!(pre instanceof HTMLPreElement)) {
        return;
      }
      var container = document.createElement("div");
      container.className = "mermaid";
      container.textContent = code.textContent || "";
      pre.replaceWith(container);
      nodes.push(container);
    });
    return nodes;
  }

  // Wrap a fenced <pre> in the frame that owns the block's external geometry,
  // mirroring the WebUI: the <pre> stays the only scroll container while the
  // frame positions the absolutely-positioned copy overlay, so scrolling a
  // long line sideways never carries the button away with the source text.
  // Idempotent: a pre already framed is returned as-is.
  function ensureCodeFrame(pre) {
    var parent = pre.parentElement;
    if (
      parent instanceof HTMLDivElement &&
      parent.classList.contains("m2h-code-frame")
    ) {
      return parent;
    }

    var frame = document.createElement("div");
    frame.className = "m2h-code-frame";
    pre.replaceWith(frame);
    frame.append(pre);
    return frame;
  }

  function addCodeCopyButtons(root) {
    root.querySelectorAll("pre").forEach(function (pre) {
      var code = pre.firstElementChild;
      if (!(code instanceof HTMLElement) || code.tagName !== "CODE") {
        return;
      }
      // Mermaid is skipped: replaceMermaidBlocks below swaps the <pre> for a
      // rendered diagram, so a frame and button would only flash before
      // dying with the pre.
      if (code.classList.contains("language-mermaid")) {
        return;
      }
      var frame = ensureCodeFrame(pre);
      if (frame.querySelector(":scope > .m2h-code-copy") !== null) {
        return;
      }

      var button = document.createElement("button");
      button.type = "button";
      button.className = "m2h-code-copy";
      button.innerHTML = COPY_ICON;
      button.setAttribute("aria-label", "复制代码");
      button.title = "复制代码";
      button.addEventListener("click", function () {
        copyCode(code.textContent || "").then(function (copied) {
          setCopyStatus(button, copied);
        });
      });
      frame.append(button);
    });
  }

  function copyCode(value) {
    // Clipboard API requires a secure context. The execCommand fallback keeps
    // the default HTTP preview usable within the click's user gesture.
    if (window.isSecureContext) {
      try {
        return navigator.clipboard.writeText(value).then(function () {
          return true;
        }, copyWithExecCommand.bind(null, value));
      } catch (_) {
        // Fall through when clipboard permission is unavailable or denied.
      }
    }
    return Promise.resolve(copyWithExecCommand(value));
  }

  function copyWithExecCommand(value) {
    var textarea = document.createElement("textarea");
    textarea.value = value;
    textarea.setAttribute("aria-hidden", "true");
    textarea.style.cssText = "position:fixed;left:-9999px;top:0;opacity:0";
    document.body.append(textarea);
    textarea.select();
    try {
      return document.execCommand("copy");
    } catch (_) {
      return false;
    } finally {
      textarea.remove();
    }
  }

  function setCopyStatus(button, copied) {
    button.innerHTML = copied ? COPIED_ICON : COPY_FAILED_ICON;
    button.dataset.copyState = copied ? "success" : "error";
    button.setAttribute(
      "aria-label",
      copied ? "代码已复制" : "复制代码失败，请手动复制"
    );
    button.title = copied ? "已复制" : "复制失败，请手动复制";
    window.setTimeout(function () {
      button.innerHTML = COPY_ICON;
      button.removeAttribute("data-copy-state");
      button.setAttribute("aria-label", "复制代码");
      button.title = "复制代码";
    }, 2000);
  }

  // Instantiate the client-side sorter on every plain GFM table with a header
  // and more than one body row; tables carrying a class attribute are
  // user-authored HTML and stay untouched. The data-m2h-sortable marker is
  // set before construction and removed again if construction throws, so a
  // failed enhancement can be retried on a later pass.
  function addSortableTables(root) {
    if (typeof Tablesort !== "function") {
      return;
    }
    root
      .querySelectorAll('table:not([class]):not([data-m2h-sortable="true"])')
      .forEach(function (table) {
        if (
          !table.tHead ||
          table.tBodies.length === 0 ||
          table.tBodies[0].rows.length <= 1
        ) {
          return;
        }
        table.dataset.m2hSortable = "true";
        prepareSortableTable(table);
        try {
          new Tablesort(table);
        } catch (_) {
          delete table.dataset.m2hSortable;
          return;
        }
        finalizeSortableTable(table);
      });
  }

  // Headers whose cells embed interactive nodes are opted out of sorting
  // before construction: Tablesort natively ignores data-sort-method="none",
  // so clicks on a link or button header keep their native meaning.
  function prepareSortableTable(table) {
    table.querySelectorAll("thead th").forEach(function (th) {
      if (
        th.querySelector(
          "a, button, input, select, textarea, [contenteditable=true]"
        ) !== null
      ) {
        th.dataset.sortMethod = "none";
      }
    });
  }

  // Tablesort writes direction to aria-sort itself but only assigns a
  // lowercase (inert) `tabindex` property, so keyboard access, the sort hint
  // title, and the explicit aria-sort="none" baseline are layered on here.
  function finalizeSortableTable(table) {
    table.addEventListener("afterSort", function () {
      syncTableSortState(table);
    });
    sortableHeaders(table).forEach(function (th) {
      th.tabIndex = 0;
      th.setAttribute("aria-sort", "none");
      th.title = "点击升序排序";
      th.addEventListener("keydown", function (event) {
        if (event.key !== "Enter" && event.key !== " ") {
          return;
        }
        event.preventDefault();
        th.click();
      });
    });
    syncTableSortState(table);
  }

  // Keep titles in step with the aria-sort attribute after every sort. When
  // another column takes over, Tablesort strips the previous column's
  // aria-sort; restoring the explicit "none" baseline keeps the header state
  // machine predictable for CSS and assistive technology alike.
  function syncTableSortState(table) {
    sortableHeaders(table).forEach(function (th) {
      var order = th.getAttribute("aria-sort");
      if (order === "ascending") {
        th.title = "当前升序，点击切换为降序";
      } else if (order === "descending") {
        th.title = "当前降序，点击切换为升序";
      } else {
        th.setAttribute("aria-sort", "none");
        th.title = "点击升序排序";
      }
    });
  }

  function sortableHeaders(table) {
    return table.querySelectorAll(
      'thead th[role="columnheader"]:not([data-sort-method="none"])'
    );
  }

  function addHeadingPermalinks(root) {
    var headings = root.querySelectorAll(
      "h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]"
    );
    Array.prototype.forEach.call(headings, function (heading) {
      var id = heading.id;
      if (id === "" || heading.querySelector(":scope > .m2h-heading-anchor")) {
        return;
      }
      var anchor = document.createElement("a");
      anchor.className = "m2h-heading-anchor";
      anchor.href = "#" + id;
      // aria-hidden keeps the icon out of the accessibility tree so it never
      // pollutes the heading's accessible name; the title still tooltips.
      anchor.setAttribute("aria-hidden", "true");
      anchor.title = "此标题的永久链接";
      anchor.innerHTML = HEADING_ANCHOR_ICON;
      heading.prepend(anchor);
    });
  }

  function decodeFragment(hash) {
    var encoded = hash.charAt(0) === "#" ? hash.slice(1) : hash;
    if (encoded === "") {
      return "";
    }
    try {
      return decodeURIComponent(encoded);
    } catch (_) {
      return encoded;
    }
  }

  // Jump to the URL fragment now that Mermaid/KaTeX/permalinks have settled, so
  // the final position is not thrown off by async rendering. Returns true when a
  // target was found and scrolled into view.
  function restoreDeepLink() {
    var id = decodeFragment(window.location.hash);
    if (id === "") {
      return false;
    }
    var target = document.getElementById(id);
    if (target === null) {
      return false;
    }
    target.scrollIntoView({ block: "start" });
    return true;
  }

  // Mirror the WebUI heading spy on the window: track H1–H6, keep the URL hash
  // in sync via replaceState (never push, so the back stack stays clean across
  // many headings), and clear it at the very top. Attached only after the
  // initial deep-link restore so it cannot clobber the fragment first.
  function setupHeadingSpy(root) {
    var headings = Array.prototype.slice.call(
      root.querySelectorAll("h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]")
    );
    if (headings.length === 0) {
      return;
    }
    var offset = 16;
    var frame = 0;
    var syncHash = function (id) {
      var next = id === null ? "" : "#" + encodeURIComponent(id);
      if (next === window.location.hash) {
        return;
      }
      var url = window.location.pathname + window.location.search + next;
      window.history.replaceState(null, "", url);
    };
    var update = function () {
      if ((window.pageYOffset || window.scrollY || 0) <= 1) {
        syncHash(null);
        return;
      }
      var current = null;
      for (var i = 0; i < headings.length; i++) {
        if (headings[i].getBoundingClientRect().top <= offset) {
          current = headings[i].id;
        } else {
          break;
        }
      }
      syncHash(current);
    };
    var handleScroll = function () {
      if (frame) {
        cancelAnimationFrame(frame);
      }
      frame = requestAnimationFrame(update);
    };
    window.addEventListener("scroll", handleScroll, { passive: true });
    handleScroll();
  }

  function enhance() {
    var root = documentRoot();
    if (root === null) {
      return;
    }

    if (typeof mermaid !== "undefined" && typeof mermaid.initialize === "function") {
      var dark = isDarkMode();
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: dark ? "dark" : "default",
      });
    }

    addHeadingPermalinks(root);
    addCodeCopyButtons(root);
    var nodes = replaceMermaidBlocks(root);
    var pending =
      typeof mermaid !== "undefined" &&
      typeof mermaid.run === "function" &&
      nodes.length > 0
        ? mermaid.run({ nodes: nodes, suppressErrors: true })
        : null;

    var afterEnhance = function () {
      renderMath(root);
      addSortableTables(root);
      restoreDeepLink();
      setupHeadingSpy(root);
    };

    if (pending && typeof pending.then === "function") {
      pending.then(afterEnhance);
    } else {
      afterEnhance();
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
