import { type RefObject, useEffect, useState } from "react";

import type { TocItem } from "./api";

// useTocSpy tracks which heading is currently active as the reader scrolls.
// The reader body scrolls inside a Base UI ScrollArea viewport (not the
// window), so the scroll listener is attached to that viewport element found
// inside the supplied container. A scroll handler throttled by
// requestAnimationFrame picks the last heading whose top has crossed the
// viewport's top edge, which is far more predictable than IntersectionObserver
// thresholds for "current section" highlighting.
export function useTocSpy<T extends HTMLElement>(
  items: TocItem[],
  containerRef: RefObject<T | null>,
  enabled: boolean,
): string | null {
  const [activeID, setActiveID] = useState<string | null>(items[0]?.id ?? null);

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
      // Offset slightly below the viewport top so a heading becomes active as
      // it approaches the toolbar rather than only once it passes it.
      const viewportTop = viewport.getBoundingClientRect().top + 16;
      let current: string | null = items[0]?.id ?? null;
      for (const item of items) {
        const heading = document.getElementById(item.id);
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

  // Keep the active heading in sync when the document (and therefore items)
  // changes while the panel is hidden: the next render shows a sensible active
  // state instead of a stale id from a previous document.
  useEffect(() => {
    if (items.length === 0) {
      setActiveID(null);
    } else {
      setActiveID((current) =>
        current !== null && items.some((item) => item.id === current)
          ? current
          : (items[0]?.id ?? null),
      );
    }
  }, [items]);

  return activeID;
}
