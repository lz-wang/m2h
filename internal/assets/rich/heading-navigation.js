// Heading permalink and scroll synchronization for generated m2h HTML.
(function () {
  "use strict";

  var HEADING_SELECTOR = "h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]";
  var HEADING_LINK_ICON =
    '<svg aria-hidden="true" focusable="false" viewBox="0 0 24 24"><path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"></path><path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"></path></svg>';

  function documentRoot() {
    return document.querySelector(".markdown-body");
  }

  function readLocationHashID() {
    var encoded = window.location.hash.slice(1);
    if (!encoded) {
      return null;
    }
    try {
      return decodeURIComponent(encoded);
    } catch (_) {
      return encoded;
    }
  }

  function replaceLocationHash(id) {
    if (readLocationHashID() === id) {
      return;
    }
    window.history.replaceState(
      window.history.state,
      "",
      "#" + encodeURIComponent(id)
    );
  }

  function restoreCurrentHash(root) {
    var id = readLocationHashID();
    if (!id) {
      return;
    }
    var target = document.getElementById(id);
    if (target && root.contains(target)) {
      target.scrollIntoView({ block: "start" });
    }
  }

  function addHeadingAnchors(root) {
    root.querySelectorAll(HEADING_SELECTOR).forEach(function (heading) {
      if (!heading.id || heading.querySelector(".m2h-heading-anchor") !== null) {
        return;
      }
      var text = (heading.textContent || "").trim();
      var anchor = document.createElement("a");
      anchor.className = "m2h-heading-anchor";
      anchor.href = "#" + encodeURIComponent(heading.id);
      anchor.innerHTML = HEADING_LINK_ICON;
      anchor.setAttribute(
        "aria-label",
        text ? "链接到标题「" + text + "」" : "链接到此标题"
      );
      anchor.title = "链接到此标题";
      anchor.addEventListener("click", function (event) {
        if (
          event.button !== 0 ||
          event.altKey ||
          event.ctrlKey ||
          event.metaKey ||
          event.shiftKey
        ) {
          return;
        }
        event.preventDefault();
        var reduceMotion =
          typeof window.matchMedia === "function" &&
          window.matchMedia("(prefers-reduced-motion: reduce)").matches;
        heading.scrollIntoView({
          block: "start",
          behavior: reduceMotion ? "auto" : "smooth",
        });
        replaceLocationHash(heading.id);
      });
      heading.prepend(anchor);
    });
  }

  function setupScrollSync(root) {
    var frame = 0;
    function update() {
      var current = null;
      root.querySelectorAll(HEADING_SELECTOR).forEach(function (heading) {
        if (heading.getBoundingClientRect().top <= 16) {
          current = heading.id;
        }
      });
      if (current) {
        replaceLocationHash(current);
      }
    }
    function handleScroll() {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(update);
    }
    window.addEventListener("scroll", handleScroll, { passive: true });
  }

  function keepInitialDeepLinkStable(root) {
    if (!readLocationHashID()) {
      return;
    }
    var frame = 0;
    function scheduleRestore() {
      window.cancelAnimationFrame(frame);
      frame = window.requestAnimationFrame(function () {
        restoreCurrentHash(root);
      });
    }
    var observer = new MutationObserver(scheduleRestore);
    observer.observe(root, { childList: true, subtree: true });
    scheduleRestore();
    window.addEventListener("load", scheduleRestore, { once: true });
    window.setTimeout(function () {
      observer.disconnect();
      scheduleRestore();
    }, 2000);
  }

  function enhance() {
    var root = documentRoot();
    if (!root) {
      return;
    }
    addHeadingAnchors(root);
    keepInitialDeepLinkStable(root);
    setupScrollSync(root);
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", enhance);
  } else {
    enhance();
  }
})();
