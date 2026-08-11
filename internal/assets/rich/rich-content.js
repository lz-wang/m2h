// m2h rich-content runtime for standalone HTML (convert + single-file preview).
//
// Mirrors web/src/lib/render-rich-content.ts: after the browser receives the
// rendered Markdown body, replace ```mermaid fenced blocks with diagram
// containers and let KaTeX scan the remaining text for math. Mermaid runs first
// so KaTeX never sees raw diagram source. Loaded as a plain <script> after
// katex.min.js, auto-render.min.js and mermaid.min.js, which attach the
// `renderMathInElement` and `mermaid` globals.
(function () {
  "use strict";

  var DELIMITERS = [
    { left: "$$", right: "$$", display: true },
    { left: "\\[", right: "\\]", display: true },
    { left: "\\(", right: "\\)", display: false },
    { left: "$", right: "$", display: false },
  ];

  function documentRoot() {
    return document.querySelector(".markdown-body");
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

  function enhance() {
    var root = documentRoot();
    if (root === null) {
      return;
    }

    if (typeof mermaid !== "undefined" && typeof mermaid.initialize === "function") {
      mermaid.initialize({
        startOnLoad: false,
        securityLevel: "strict",
        theme: "neutral",
      });
    }

    var nodes = replaceMermaidBlocks(root);
    var pending =
      typeof mermaid !== "undefined" &&
      typeof mermaid.run === "function" &&
      nodes.length > 0
        ? mermaid.run({ nodes: nodes, suppressErrors: true })
        : null;

    if (pending && typeof pending.then === "function") {
      pending.then(function () {
        renderMath(root);
      });
    } else {
      renderMath(root);
    }
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
