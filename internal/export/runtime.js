// m2h export runtime: the minimal enhancer inlined into every exported page.
// It renders Mermaid diagrams, KaTeX math and sortable tables. Lightbox,
// line numbers, code collapse and theme switching belong to the WebUI.
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
    "script, noscript, style, textarea, pre, code, option, .katex, .m2h-math, ." + LITERAL_DOLLAR_CLASS;
  var ZENUML_SOURCE = /^\s*zenuml\b/;

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

  // The resolved theme, computed once: an explicit m2h-mode-* class wins,
  // otherwise auto follows the system preference for Mermaid's theme.
  function resolveDark() {
    var rootClasses = document.documentElement.classList;
    return rootClasses.contains("m2h-mode-dark") ||
      (!rootClasses.contains("m2h-mode-light") &&
        typeof window.matchMedia === "function" &&
        window.matchMedia("(prefers-color-scheme: dark)").matches);
  }

  function enhance() {
    var root = document.querySelector(".markdown-body");
    if (root === null) {
      return;
    }
    var hasMermaid = typeof mermaid !== "undefined" && typeof mermaid.initialize === "function";
    if (hasMermaid) {
      // Prevent the CDN window-load listener from scanning containers twice.
      mermaid.startOnLoad = false;
    }
    var nodes = [];
    if (hasMermaid) {
      root.querySelectorAll("pre > code.language-mermaid").forEach(function (code) {
        if (ZENUML_SOURCE.test(code.textContent || "")) {
          return;
        }
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
    var dark = resolveDark();
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
        return;
      }
      mermaid.initialize({ startOnLoad: false, securityLevel: "strict", theme: dark ? "dark" : "default" });
    };
    var runDiagrams = function () {
      var pending = hasMermaid && typeof mermaid.run === "function" && nodes.length > 0
        ? mermaid.run({ nodes: nodes, suppressErrors: true })
        : null;
      Promise.resolve(pending).then(finish, finish);
    };
    initializeTheme();
    runDiagrams();
  }
  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
