import { act, fireEvent, render, screen } from "@testing-library/react";
import { useRef } from "react";
import { describe, expect, it, vi } from "vitest";

import type { TocItem } from "./api";
import { useTocSpy } from "./use-toc-spy";

const items: TocItem[] = [
  { level: 2, id: "a", text: "A" },
  { level: 2, id: "b", text: "B" },
];

function Harness({ enabled }: { enabled: boolean }) {
  const ref = useRef<HTMLDivElement>(null);
  const activeID = useTocSpy(items, ref, enabled);
  return (
    <div ref={ref}>
      <div data-slot="scroll-area-viewport" data-testid="viewport">
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

describe("useTocSpy", () => {
  it("activates the last heading scrolled past the viewport top", async () => {
    // "a" sits at the top, "b" is far below the offset → only "a" is active.
    mockHeadingTops({ a: 0, b: 400 });
    render(<Harness enabled={true} />);
    expect(screen.getByTestId("active").textContent).toBe("a");

    // After scrolling, "b" crosses the offset and becomes active.
    mockHeadingTops({ a: -200, b: 0 });
    await act(async () => {
      fireEvent.scroll(screen.getByTestId("viewport"));
      await new Promise((resolve) => {
        setTimeout(resolve, 0);
      });
    });
    expect(screen.getByTestId("active").textContent).toBe("b");
  });

  it("leaves the initial heading active while disabled", () => {
    render(<Harness enabled={false} />);
    expect(screen.getByTestId("active").textContent).toBe("a");
    // No viewport listener is attached when disabled, so scrolling is a no-op.
    fireEvent.scroll(screen.getByTestId("viewport"));
    expect(screen.getByTestId("active").textContent).toBe("a");
  });
});
