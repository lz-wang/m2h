import { act, fireEvent, render, screen } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { TocItem } from "./api";
import { useHeadingSpy } from "./use-heading-spy";

const items: TocItem[] = [
  { level: 1, id: "title", text: "Title" },
  { level: 2, id: "a", text: "A" },
  { level: 2, id: "b", text: "B" },
];

function Harness({ enabled }: { enabled: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const activeID = useHeadingSpy(items, ref, enabled);
  return (
    <div ref={ref}>
      <div className="markdown-body">
        <h1 id="title">Title</h1>
        <h2 id="a">A</h2>
        <h2 id="b">B</h2>
      </div>
      <span data-testid="active">{activeID ?? "none"}</span>
    </div>
  );
}

function mockHeadingTops(tops: Record<string, number>): void {
  vi.spyOn(Element.prototype, "getBoundingClientRect").mockImplementation(
    function (this: Element) {
      const top = this.id.length > 0 ? (tops[this.id] ?? 0) : 0;
      return {
        top,
        bottom: 0,
        left: 0,
        right: 0,
        width: 0,
        height: 0,
        x: 0,
        y: 0,
        toJSON() {},
      } as DOMRect;
    },
  );
}

// jsdom does not lay out, so window.scrollY stays 0: set it explicitly to
// model a position partway down the document.
function windowScrollY(value: number): void {
  Object.defineProperty(window, "scrollY", {
    configurable: true,
    value,
  });
}

async function fireScroll(): Promise<void> {
  await act(async () => {
    fireEvent.scroll(window);
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
  });
}

describe("useHeadingSpy", () => {
  beforeEach(() => {
    windowScrollY(0);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("does not compute an initial position on mount", () => {
    // The window already sits mid-document (the browser's native scroll
    // restoration may land there before React hydrates), but the spy must not
    // report anything until a real scroll event arrives — an eager first pass
    // would pin the URL to a heading while the restoration is still settling.
    mockHeadingTops({ title: -400, a: 0, b: 400 });
    windowScrollY(100);
    render(<Harness enabled={true} />);
    expect(screen.getByTestId("active").textContent).toBe("none");
  });

  it("clears the active heading while pinned to the top", async () => {
    mockHeadingTops({ title: -400, a: 0, b: 400 });
    render(<Harness enabled={true} />);
    windowScrollY(100);
    await fireScroll();
    expect(screen.getByTestId("active").textContent).toBe("a");

    // Scrolling back to the very top clears the section so the URL stays
    // clean instead of pinning the first heading.
    mockHeadingTops({ title: 0, a: 200, b: 400 });
    windowScrollY(0);
    await fireScroll();
    expect(screen.getByTestId("active").textContent).toBe("none");
  });

  it("activates the last heading scrolled past the toolbar edge", async () => {
    mockHeadingTops({ title: -400, a: 0, b: 400 });
    render(<Harness enabled={true} />);
    windowScrollY(100);
    await fireScroll();
    expect(screen.getByTestId("active").textContent).toBe("a");

    // After scrolling further, "b" crosses the offset and becomes active.
    mockHeadingTops({ title: -600, a: -200, b: 0 });
    await fireScroll();
    expect(screen.getByTestId("active").textContent).toBe("b");
  });

  it("does not attach a listener while disabled", async () => {
    mockHeadingTops({ title: 0, a: 0, b: 0 });
    render(<Harness enabled={false} />);
    windowScrollY(100);
    await fireScroll();
    expect(screen.getByTestId("active").textContent).toBe("none");
  });
});
