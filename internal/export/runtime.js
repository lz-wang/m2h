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
  function enhance() {
    var root = document.querySelector(".markdown-body");
    if (root === null) {
      return;
    }
    var nodes = [];
    if (typeof mermaid !== "undefined" && typeof mermaid.initialize === "function") {
      var rootClasses = document.documentElement.classList;
      var dark = rootClasses.contains("m2h-mode-dark") ||
        (!rootClasses.contains("m2h-mode-light") &&
          typeof window.matchMedia === "function" &&
          window.matchMedia("(prefers-color-scheme: dark)").matches);
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default" });
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
    var pending = typeof mermaid !== "undefined" && typeof mermaid.run === "function" && nodes.length > 0
      ? mermaid.run({ nodes: nodes, suppressErrors: true })
      : null;
    if (pending && typeof pending.then === "function") {
      pending.then(finish);
    } else {
      finish();
    }
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
