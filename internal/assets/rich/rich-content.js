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
  var COPY_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><rect x="5.25" y="5.25" width="7.25" height="7.25" rx="1.25"></rect><path d="M10.75 5.25V3.5c0-.69-.56-1.25-1.25-1.25H3.5c-.69 0-1.25.56-1.25 1.25v6c0 .69.56 1.25 1.25 1.25h1.75"></path></svg>';
  var COPIED_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m3.5 8.25 2.1 2.1 4.9-4.9"></path></svg>';
  var COPY_FAILED_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 16 16"><path d="m4.5 4.5 7 7m0-7-7 7"></path></svg>';

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

  function addCodeCopyButtons(root) {
    root.querySelectorAll("pre").forEach(function (pre) {
      var code = pre.firstElementChild;
      if (!(code instanceof HTMLElement) || code.tagName !== "CODE") {
        return;
      }
      if (pre.querySelector(".m2h-code-copy") !== null) {
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
      pre.append(button);
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

    addCodeCopyButtons(root);
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
