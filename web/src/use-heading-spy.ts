import { type RefObject, useEffect, useState } from "react";

import type { TocItem } from "./api";
import { findHeadingElement } from "./lib/heading";

// useHeadingSpy tracks the heading currently in view as the reader scrolls.
//
// Unlike a TOC-specific spy it follows ALL headings (H1–H6) whenever a document
// is ready, independent of whether the right-hand TOC panel is shown: the URL
// must reflect the reading position even with the TOC hidden. The caller is free
// to derive the narrower "TOC active" highlight from this single source (see
// App.tsx), so only one scroll listener ever runs.
//
// The document scrolls in the window itself (that is what makes the browser's
// native scroll restoration reliable), so the listener attaches to window and
// reads window.scrollY. A scroll handler throttled by requestAnimationFrame
// picks the last heading whose top has crossed the activation line — the bottom
// edge of the sticky toolbar, plus a little lead so a heading becomes active as
// it approaches the bar rather than only once it passes under it.
//
// The spy only reacts to real scroll events — it never computes an initial
// position on mount. The browser's own scroll restoration finishes by moving
// the viewport (which fires scroll), and only then does the hash resync; an
// eager first pass would pin the URL to a wrong heading while the restore is
// still settling.
//
// At the very top of the document (scrollY ≈ 0) no heading is active so the URL
// stays clean instead of snapping to the first heading the moment the page
// opens.
export function useHeadingSpy<T extends HTMLElement>(
  items: TocItem[],
  containerRef: RefObject<T | null>,
  enabled: boolean,
): string | null {
  const [activeID, setActiveID] = useState<string | null>(null);

  useEffect(() => {
    if (!enabled || items.length === 0) {
      return;
    }
    const container = containerRef.current;
    if (container === null) {
      return;
    }
    // The toolbar is a sibling of the reader container, so reach it through
    // the shared inset. Missing in reduced harnesses (tests): fall back to the
    // viewport top.
    const toolbar = container
      .closest(".reader-inset")
      ?.querySelector<HTMLElement>(".reader-toolbar");

    let frame = 0;
    const update = () => {
      // While pinned to the top of the document there is no "current section":
      // keep the active id (and therefore the URL hash) clear.
      if (window.scrollY <= 1) {
        setActiveID(null);
        return;
      }
      const viewportTop = (toolbar?.getBoundingClientRect().bottom ?? 0) + 16;
      let current: string | null = null;
      for (const item of items) {
        const heading = findHeadingElement(container, item.id);
        if (heading === null) {
          continue;
        }
        if (heading.getBoundingClientRect().top <= viewportTop) {
          current = item.id;
          continue;
        }
        break;
      }
      setActiveID(current);
    };
    const handleScroll = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(update);
    };

    window.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("scroll", handleScroll);
    };
  }, [items, containerRef, enabled]);

  // Drop a stale id the instant the heading set changes (a new document), so the
  // next paint shows a clean state until the scroll handler recomputes from the
  // new positions.
  useEffect(() => {
    setActiveID((current) =>
      current !== null && items.some((item) => item.id === current)
        ? current
        : null,
    );
  }, [items]);

  return activeID;
}
