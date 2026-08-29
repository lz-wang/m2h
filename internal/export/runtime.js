// m2h export runtime: the minimal enhancer inlined into every exported page.
// It renders Mermaid diagrams, KaTeX math, and sortable tables — nothing else.
// Lightbox, line numbers, code collapse, heading spy, share, and theme
// switching belong to the WebUI; exported pages deliberately stay this small.
(function () {
  "use strict";
  var DELIMITERS = [
    { left: "$$", right: "$$", display: true },
    { left: "\\[", right: "\\]", display: true },
    { left: "\\(", right: "\\)", display: false },
    { left: "$", right: "$", display: false }
  ];
  // KaTeX auto-render pairs every two single dollars without Markdown's usual
  // flanking rules. Split unmatched dollars into ignored spans so currency such
  // as "$9 ... $200" stays literal while $x$ keeps rendering as inline math.
  var LITERAL_DOLLAR_CLASS = "m2h-literal-dollar";
  var KATEX_IGNORED_CONTENT_SELECTOR =
    "script, noscript, style, textarea, pre, code, option, .katex, ." + LITERAL_DOLLAR_CLASS;
  // Same leading-keyword rule as the official plugin's own detector: only a
  // diagram whose source starts with the zenuml keyword needs the plugin.
  var ZENUML_PREFIX = /^\s*zenuml/;
  // @zenuml/core returns one static light SVG palette. Keep the dark correction
  // inside that SVG so exported pages match Mermaid's ordinary dark sequence
  // diagrams without installing another host stylesheet.
  var ZENUML_DARK_THEME_STYLE = [
    'svg[data-m2h-zenuml-theme="dark"] { color-scheme: dark; }',
    'svg[data-m2h-zenuml-theme="dark"] .frame-border-outer { fill: #d3d3d3; }',
    'svg[data-m2h-zenuml-theme="dark"] .frame-border-inner, svg[data-m2h-zenuml-theme="dark"] .frame-header-bg, svg[data-m2h-zenuml-theme="dark"] .participant-box, svg[data-m2h-zenuml-theme="dark"] .group-title-bg { fill: #1f2020; }',
    'svg[data-m2h-zenuml-theme="dark"] .frame-header-line, svg[data-m2h-zenuml-theme="dark"] .participant-box, svg[data-m2h-zenuml-theme="dark"] .participant-icon [fill="currentColor"]:not([stroke]), svg[data-m2h-zenuml-theme="dark"] .lifeline, svg[data-m2h-zenuml-theme="dark"] .fragment-border, svg[data-m2h-zenuml-theme="dark"] .group-outline { stroke: #d3d3d3; }',
    'svg[data-m2h-zenuml-theme="dark"] .frame-title, svg[data-m2h-zenuml-theme="dark"] .participant-label, svg[data-m2h-zenuml-theme="dark"] .message-label, svg[data-m2h-zenuml-theme="dark"] .fragment-label, svg[data-m2h-zenuml-theme="dark"] .fragment-condition, svg[data-m2h-zenuml-theme="dark"] .fragment-section-label, svg[data-m2h-zenuml-theme="dark"] .return-label, svg[data-m2h-zenuml-theme="dark"] .return-icon, svg[data-m2h-zenuml-theme="dark"] .group-title-text { fill: #cccccc; }',
    'svg[data-m2h-zenuml-theme="dark"] .participant-icon { color: #cccccc; }',
    'svg[data-m2h-zenuml-theme="dark"] .message-line, svg[data-m2h-zenuml-theme="dark"] .arrow-head, svg[data-m2h-zenuml-theme="dark"] .return-line, svg[data-m2h-zenuml-theme="dark"] .return-arrow, svg[data-m2h-zenuml-theme="dark"] .arrow-head path[stroke] { stroke: #cccccc; }',
    'svg[data-m2h-zenuml-theme="dark"] .arrow-head:not(.arrow-open) { fill: #cccccc; }',
    'svg[data-m2h-zenuml-theme="dark"] .arrow-open, svg[data-m2h-zenuml-theme="dark"] .return-arrow { fill: none; }',
    'svg[data-m2h-zenuml-theme="dark"] .occurrence, svg[data-m2h-zenuml-theme="dark"] .fragment-header { fill: #474949; }',
    'svg[data-m2h-zenuml-theme="dark"] .occurrence { stroke: #d3d3d3; }',
    'svg[data-m2h-zenuml-theme="dark"] .fragment-separator { stroke: #2f2f2f; }',
    'svg[data-m2h-zenuml-theme="dark"] .divider-bg { fill: #2f2f2f; stroke: #aaaa33; }',
    'svg[data-m2h-zenuml-theme="dark"] .divider-label { fill: #d3d3d3; }',
    'svg[data-m2h-zenuml-theme="dark"] .comment-text, svg[data-m2h-zenuml-theme="dark"] .seq-number { fill: #b8b6b6; }'
  ].join("\n");

  // Mermaid Core does not know the zenuml diagram type; the plugin module
  // (whose pinned URL the page carries in window.m2hZenUMLModuleURL) must be
  // dynamically imported and registered before initialize. A failure only
  // degrades ZenUML blocks — plain diagrams keep rendering — so it is reported
  // and swallowed here instead of aborting the whole enhancer.
  function hostStylesheets() {
    return new Set(Array.from(document.head.children).filter(function (element) {
      return element instanceof HTMLStyleElement ||
        (element instanceof HTMLLinkElement && element.rel === "stylesheet");
    }));
  }

  // External renderers may return SVG, but may not leave global styles behind.
  // The pinned ZenUML browser bundle injects an unscoped stylesheet while its
  // diagram chunk registers; the native SVG output already includes the local
  // styles it needs. Clean additions on both success and failure while keeping
  // every stylesheet that belonged to the exported page before registration.
  function withoutAddedHostStylesheets(operation) {
    var retained = hostStylesheets();
    var cleanup = function () {
      hostStylesheets().forEach(function (stylesheet) {
        if (!retained.has(stylesheet)) {
          stylesheet.remove();
        }
      });
    };
    return Promise.resolve().then(operation).then(
      function (value) {
        cleanup();
        return value;
      },
      function (error) {
        cleanup();
        throw error;
      }
    );
  }

  function registerZenUML() {
    var url = window.m2hZenUMLModuleURL;
    if (!url || typeof mermaid.registerExternalDiagrams !== "function") {
      console.warn("ZenUML diagram present but no plugin module URL is available");
      return Promise.resolve();
    }
    return withoutAddedHostStylesheets(function () {
      return import(url).then(function (plugin) {
        return mermaid.registerExternalDiagrams([plugin.default], { lazyLoad: false });
      });
    })
      .catch(function (error) {
        console.warn("Failed to register the ZenUML diagram plugin", error);
      });
  }

  function applyZenUMLTheme(root, dark) {
    root.querySelectorAll('.mermaid[data-m2h-engine="zenuml"] > svg').forEach(function (svg) {
      var mode = dark ? "dark" : "light";
      svg.setAttribute("data-m2h-zenuml-theme", mode);
      if (!dark) {
        return;
      }
      var definitions = svg.querySelector(":scope > defs");
      if (!definitions) {
        definitions = document.createElementNS("http://www.w3.org/2000/svg", "defs");
        svg.insertBefore(definitions, svg.firstChild);
      }
      var style = document.createElementNS("http://www.w3.org/2000/svg", "style");
      style.setAttribute("data-m2h-zenuml-theme-style", "dark");
      style.textContent = ZENUML_DARK_THEME_STYLE;
      definitions.appendChild(style);
    });
  }

  function literalDollarIndexes(source) {
    var singles = [];
    var openers = [];
    var matched = new Set();
    for (var index = 0; index < source.length; index++) {
      if (source[index] !== "$" || source[index - 1] === "$" || source[index + 1] === "$") {
        continue;
      }
      singles.push(index);
      var previous = source[index - 1] || "";
      var next = source[index + 1] || "";
      var canOpen = next !== "" && !/\s/.test(next);
      var canClose = previous !== "" && !/\s/.test(previous) && !/[0-9]/.test(next);
      if (canClose && openers.length > 0) {
        matched.add(openers.pop());
        matched.add(index);
      } else if (canOpen) {
        openers.push(index);
      }
    }
    return singles.filter(function (index) { return !matched.has(index); });
  }

  function protectLiteralDollars(root) {
    var walker = document.createTreeWalker(root, NodeFilter.SHOW_TEXT);
    var candidates = [];
    for (var node = walker.nextNode(); node !== null; node = walker.nextNode()) {
      candidates.push(node);
    }
    candidates.forEach(function (node) {
      var parent = node.parentElement;
      var source = node.data;
      if (!parent || source.indexOf("$") === -1 || parent.closest(KATEX_IGNORED_CONTENT_SELECTOR)) {
        return;
      }
      var literalIndexes = literalDollarIndexes(source);
      if (literalIndexes.length === 0) {
        return;
      }
      var fragment = document.createDocumentFragment();
      var start = 0;
      literalIndexes.forEach(function (index) {
        fragment.append(source.slice(start, index));
        var literal = document.createElement("span");
        literal.className = LITERAL_DOLLAR_CLASS;
        literal.textContent = "$";
        fragment.append(literal);
        start = index + 1;
      });
      fragment.append(source.slice(start));
      node.replaceWith(fragment);
    });
  }

  function enhance() {
    var root = document.querySelector(".markdown-body");
    if (root === null) {
      return;
    }
    var hasMermaid = typeof mermaid !== "undefined" && typeof mermaid.initialize === "function";
    if (hasMermaid) {
      // Mermaid's CDN build also owns a window-load listener. ZenUML plugin
      // registration is asynchronous, so prevent that listener from scanning
      // the freshly-created containers before the external diagram exists.
      // initializeTheme runs only after registration and keeps startOnLoad off.
      mermaid.startOnLoad = false;
    }
    var nodes = [];
    if (hasMermaid) {
      root.querySelectorAll("pre > code.language-mermaid").forEach(function (code) {
        var pre = code.parentElement;
        if (!(pre instanceof HTMLPreElement)) {
          return;
        }
        var container = document.createElement("div");
        container.className = "mermaid";
        container.textContent = code.textContent || "";
        if (ZENUML_PREFIX.test(container.textContent)) {
          container.setAttribute("data-m2h-engine", "zenuml");
        }
        pre.replaceWith(container);
        nodes.push(container);
      });
    }
    var finish = function () {
      if (typeof renderMathInElement === "function") {
        protectLiteralDollars(root);
        renderMathInElement(root, {
          delimiters: DELIMITERS,
          ignoredClasses: [LITERAL_DOLLAR_CLASS],
          throwOnError: false
        });
      }
      if (typeof Tablesort === "function") {
        root.querySelectorAll('table:not([class]):not([data-m2h-sortable="true"])').forEach(function (table) {
          if (!table.tHead || table.tBodies.length === 0 || table.tBodies[0].rows.length <= 1) {
            return;
          }
          table.setAttribute("data-m2h-sortable", "true");
          new Tablesort(table);
        });
      }
    };
    var initializeTheme = function () {
      if (!hasMermaid) {
        return false;
      }
      var rootClasses = document.documentElement.classList;
      var dark = rootClasses.contains("m2h-mode-dark") ||
        (!rootClasses.contains("m2h-mode-light") &&
          typeof window.matchMedia === "function" &&
          window.matchMedia("(prefers-color-scheme: dark)").matches);
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default" });
      return dark;
    };
    var runDiagrams = function (dark) {
      var pending = hasMermaid && typeof mermaid.run === "function" && nodes.length > 0
        ? mermaid.run({ nodes: nodes, suppressErrors: true })
        : null;
      if (pending && typeof pending.then === "function") {
        pending.then(function () {
          applyZenUMLTheme(root, dark);
          finish();
        });
      } else {
        applyZenUMLTheme(root, dark);
        finish();
      }
    };
    var needsZenUML = hasMermaid && nodes.some(function (node) {
      return ZENUML_PREFIX.test(node.textContent || "");
    });
    // Mermaid's integration order for external diagrams: the plugin must be
    // registered before initialize configures the runtime.
    if (needsZenUML) {
      registerZenUML().then(function () {
        runDiagrams(initializeTheme());
      });
    } else {
      runDiagrams(initializeTheme());
    }
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
