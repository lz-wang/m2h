import { type RefObject, useCallback } from "react";

import { findHeadingElement } from "./lib/heading";

export interface NavigateToHeadingOptions {
  // "smooth" for user-driven clicks, "auto" (instant) for deep-link restore.
  // prefers-reduced-motion always collapses to instant scrolling.
  behavior?: ScrollBehavior;
  // When false the URL hash is left untouched (e.g. restoring a fragment that
  // is already in the address bar). Defaults to true.
  updateURL?: boolean;
}

export type NavigateToHeading = (
  id: string,
  options?: NavigateToHeadingOptions,
) => void;

// useHeadingNavigation returns a stable callback that scrolls the reader to a
// heading id resolved strictly within the rendered Markdown body, and optionally
// rewrites the URL hash through the supplied funnel. Centralizing this means the
// TOC, Markdown fragment links, heading permalinks and deep-link restore all
// share one scroll path — and a document heading id can never resolve to an
// unrelated element on the WebUI shell.
export function useHeadingNavigation(
  containerRef: RefObject<HTMLElement | null>,
  updateHash: (id: string | null) => void,
): NavigateToHeading {
  return useCallback(
    (id, options = {}) => {
      const container = containerRef.current;
      if (container === null) {
        return;
      }
      const reduceMotion =
        typeof window.matchMedia === "function" &&
        window.matchMedia("(prefers-reduced-motion: reduce)").matches;
      const heading = findHeadingElement(container, id);
      if (heading !== null) {
        heading.scrollIntoView({
          block: "start",
          behavior:
            options.behavior === "smooth" && !reduceMotion ? "smooth" : "auto",
        });
      }
      if (options.updateURL !== false) {
        updateHash(id);
      }
    },
    [containerRef, updateHash],
  );
}
