import { fireEvent, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ScrollArea } from "./scroll-area";

// The transient scrollbar is contract-tested through DOM attributes, not the
// faded opacity: jsdom applies no CSS, and the component's real job is to flip
// data-m2h-scrolling on the root (per scroll event, no React state) and remove
// it again after the grace period. The CSS layer only reacts to those flags.
describe("ScrollArea scrollbarVisibility", () => {
  beforeEach(() => {
    // Only the timeout APIs the grace period uses: the shared setup file
    // installs a read-only requestAnimationFrame stub that a full fake-timer
    // install would try to replace.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  function renderRoot(ui: React.ReactElement): HTMLElement {
    const { container } = render(ui);
    const root = container.querySelector<HTMLElement>(
      '[data-slot="scroll-area"]',
    );
    if (root === null) {
      throw new Error("scroll area root was not rendered");
    }
    return root;
  }

  it("defaults to an always-visible scrollbar without scroll handling", () => {
    const root = renderRoot(
      <ScrollArea>
        <p>content</p>
      </ScrollArea>,
    );
    expect(root.dataset.scrollbarVisibility).toBe("always");
    expect(root.dataset.m2hScrolling).toBeUndefined();
  });

  it("flags the root while scrolling and clears it after the grace period", () => {
    const root = renderRoot(
      <ScrollArea scrollbarVisibility="scrolling">
        <p>content</p>
      </ScrollArea>,
    );
    expect(root.dataset.scrollbarVisibility).toBe("scrolling");
    expect(root.dataset.m2hScrolling).toBeUndefined();

    const viewport = root.querySelector<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (viewport === null) {
      throw new Error("scroll area viewport was not rendered");
    }
    fireEvent.scroll(viewport);
    expect(root.dataset.m2hScrolling).toBe("true");

    // Halfway through the grace period another scroll re-arms the timer.
    vi.advanceTimersByTime(400);
    fireEvent.scroll(viewport);
    vi.advanceTimersByTime(400);
    expect(root.dataset.m2hScrolling).toBe("true");

    vi.advanceTimersByTime(300);
    expect(root.dataset.m2hScrolling).toBeUndefined();
  });
});
