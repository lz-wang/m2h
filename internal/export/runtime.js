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
  // Same leading-keyword rule as the official plugin's own detector: only a
  // diagram whose source starts with the zenuml keyword needs the plugin.
  var ZENUML_PREFIX = /^\s*zenuml/;

  // Mermaid Core does not know the zenuml diagram type; the plugin module
  // (whose pinned URL the page carries in window.m2hZenUMLModuleURL) must be
  // dynamically imported and registered before initialize. A failure only
  // degrades ZenUML blocks — plain diagrams keep rendering — so it is reported
  // and swallowed here instead of aborting the whole enhancer.
  function registerZenUML() {
    var url = window.m2hZenUMLModuleURL;
    if (!url || typeof mermaid.registerExternalDiagrams !== "function") {
      console.warn("ZenUML diagram present but no plugin module URL is available");
      return Promise.resolve();
    }
    return import(url)
      .then(function (plugin) {
        return mermaid.registerExternalDiagrams([plugin.default], { lazyLoad: false });
      })
      .catch(function (error) {
        console.warn("Failed to register the ZenUML diagram plugin", error);
      });
  }

  function enhance() {
    var root = document.querySelector(".markdown-body");
    if (root === null) {
      return;
    }
    var hasMermaid = typeof mermaid !== "undefined" && typeof mermaid.initialize === "function";
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
        pre.replaceWith(container);
        nodes.push(container);
      });
    }
    var finish = function () {
      if (typeof renderMathInElement === "function") {
        renderMathInElement(root, { delimiters: DELIMITERS, throwOnError: false });
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
        return;
      }
      var rootClasses = document.documentElement.classList;
      var dark = rootClasses.contains("m2h-mode-dark") ||
        (!rootClasses.contains("m2h-mode-light") &&
          typeof window.matchMedia === "function" &&
          window.matchMedia("(prefers-color-scheme: dark)").matches);
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default" });
    };
    var runDiagrams = function () {
      var pending = hasMermaid && typeof mermaid.run === "function" && nodes.length > 0
        ? mermaid.run({ nodes: nodes, suppressErrors: true })
        : null;
      if (pending && typeof pending.then === "function") {
        pending.then(finish);
      } else {
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
        initializeTheme();
        runDiagrams();
      });
    } else {
      initializeTheme();
      runDiagrams();
    }
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
