import { type RefObject, useEffect, useState } from "react";

import type { TocItem } from "./api";

const documentHeadingSelector =
  ".reader-document h1[id], .reader-document h2[id], .reader-document h3[id], .reader-document h4[id], .reader-document h5[id], .reader-document h6[id]";

// useTocSpy serves two related but deliberately independent jobs:
// 1. keep the optional H2-H4 TOC highlight in sync with the reader viewport;
// 2. keep the browser fragment in sync with every H1-H6 heading while reading.
//
// The reader body scrolls inside a Base UI ScrollArea viewport (not the
// window), so the scroll listener is attached to that viewport element found
// inside the supplied container. URL updates use history.replaceState so normal
// reading never creates hundreds of Back-button entries.
export function useTocSpy<T extends HTMLElement>(
  items: TocItem[],
  containerRef: RefObject<T | null>,
  enabled: boolean,
): string | null {
  const [activeID, setActiveID] = useState<string | null>(items[0]?.id ?? null);

  useEffect(() => {
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
    const update = (syncHash: boolean) => {
      // Offset slightly below the viewport top so a heading becomes active as
      // it approaches the toolbar rather than only once it passes it.
      const viewportTop = viewport.getBoundingClientRect().top + 16;

      if (enabled && items.length > 0) {
        let currentTOC: string | null = items[0]?.id ?? null;
        for (const item of items) {
          const heading = document.getElementById(item.id);
          if (heading === null) {
            continue;
          }
          if (heading.getBoundingClientRect().top <= viewportTop) {
            currentTOC = item.id;
            continue;
          }
          break;
        }
        setActiveID(currentTOC);
      }

      if (!syncHash) {
        return;
      }
      let currentHeading: string | null = null;
      for (const heading of document.querySelectorAll<HTMLElement>(
        documentHeadingSelector,
      )) {
        if (heading.getBoundingClientRect().top <= viewportTop) {
          currentHeading = heading.id;
          continue;
        }
        break;
      }
      if (currentHeading !== null) {
        replaceLocationHash(currentHeading);
      }
    };
    const handleScroll = () => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(() => update(true));
    };

    // Never rewrite an incoming deep-link hash during setup. The document
    // renderer first restores that fragment; the resulting/user scroll then
    // drives normal URL synchronization.
    update(false);
    viewport.addEventListener("scroll", handleScroll, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      viewport.removeEventListener("scroll", handleScroll);
    };
  }, [items, containerRef, enabled]);

  // Keep the active TOC heading sensible when the document changes while the
  // panel is hidden. URL synchronization above remains active regardless of
  // whether the TOC panel itself is visible.
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

function replaceLocationHash(id: string): void {
  if (readLocationHashID() === id) {
    return;
  }
  window.history.replaceState(
    window.history.state,
    "",
    `#${encodeURIComponent(id)}`,
  );
}

function readLocationHashID(): string | null {
  const encoded = window.location.hash.slice(1);
  if (encoded === "") {
    return null;
  }
  try {
    return decodeURIComponent(encoded);
  } catch {
    return encoded;
  }
}
