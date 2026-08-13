import { act, fireEvent, render, screen } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

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
      <div data-slot="scroll-area-viewport" data-testid="viewport">
        <div className="markdown-body">
          <h1 id="title">Title</h1>
          <h2 id="a">A</h2>
          <h2 id="b">B</h2>
        </div>
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

function scrollTop(node: HTMLElement, value: number): void {
  // jsdom does not reflect scrollTop from layout, so set it explicitly to model
  // a position partway down the document.
  Object.defineProperty(node, "scrollTop", { configurable: true, value });
}

describe("useHeadingSpy", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("clears the active heading while pinned to the top", () => {
    mockHeadingTops({ title: 0, a: 0, b: 0 });
    render(<Harness enabled={true} />);
    // scrollTop is 0 → no section is active so the URL can stay clean.
    expect(screen.getByTestId("active").textContent).toBe("none");
  });

  it("activates the last heading scrolled past the viewport top", async () => {
    mockHeadingTops({ title: -400, a: 0, b: 400 });
    render(<Harness enabled={true} />);
    const viewport = screen.getByTestId("viewport");
    scrollTop(viewport, 100);
    await act(async () => {
      fireEvent.scroll(viewport);
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(screen.getByTestId("active").textContent).toBe("a");

    // After scrolling further, "b" crosses the offset and becomes active.
    mockHeadingTops({ title: -600, a: -200, b: 0 });
    await act(async () => {
      fireEvent.scroll(viewport);
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(screen.getByTestId("active").textContent).toBe("b");
  });

  it("does not attach a listener while disabled", () => {
    mockHeadingTops({ title: 0, a: 0, b: 0 });
    render(<Harness enabled={false} />);
    const viewport = screen.getByTestId("viewport");
    scrollTop(viewport, 100);
    fireEvent.scroll(viewport);
    expect(screen.getByTestId("active").textContent).toBe("none");
  });
});
