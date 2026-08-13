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
// The reader body scrolls inside a Base UI ScrollArea viewport (not the
// window), so the scroll listener attaches to that viewport found inside the
// supplied container. A scroll handler throttled by requestAnimationFrame picks
// the last heading whose top has crossed the viewport edge, which is far more
// predictable than IntersectionObserver thresholds for "current section".
//
// At the very top of the document (scrollTop ≈ 0) no heading is active so the
// URL stays clean instead of snapping to the first heading the moment the page
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
    const viewport = container.querySelector<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport === null) {
      return;
    }

    let frame = 0;
    const update = () => {
      // While pinned to the top of the document there is no "current section":
      // keep the active id (and therefore the URL hash) clear.
      if (viewport.scrollTop <= 1) {
        setActiveID(null);
        return;
      }
      // Offset slightly below the viewport top so a heading becomes active as
      // it approaches the toolbar rather than only once it passes it.
      const viewportTop = viewport.getBoundingClientRect().top + 16;
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

    update();
    viewport.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      viewport.removeEventListener("scroll", handleScroll);
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
