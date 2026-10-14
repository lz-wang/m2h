import { act, render } from "@testing-library/react";
import { useRef } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { useHeadingNavigation } from "./use-heading-navigation";

function Harness({
  updateHash,
  id,
  options,
}: {
  updateHash: (id: string | null) => void;
  id: string;
  options?: Parameters<ReturnType<typeof useHeadingNavigation>>[1];
}) {
  const ref = useRef<HTMLDivElement>(null);
  const navigate = useHeadingNavigation(ref, updateHash);
  return (
    <div ref={ref}>
      <button
        type="button"
        onClick={() => navigate(id, options)}
        data-testid="go"
      >
        go
      </button>
      <div className="markdown-body">
        <div className="m2h-tabbed-set">
          <input
            type="radio"
            className="m2h-tabbed-radio"
            name="nav"
            id="tab-a"
            defaultChecked
          />
          <input
            type="radio"
            className="m2h-tabbed-radio"
            name="nav"
            id="tab-b"
          />
          <div className="m2h-tabbed-content">
            <section className="m2h-tabbed-panel" data-m2h-tab-id="tab-b">
              <h2 id="target">Target</h2>
            </section>
          </div>
        </div>
      </div>
    </div>
  );
}

function matchMedia(reduceMotion: boolean) {
  vi.spyOn(window, "matchMedia").mockReturnValue({
    matches: reduceMotion,
  } as MediaQueryList);
}

describe("useHeadingNavigation", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("scrolls the in-body heading to the top and rewrites the hash", () => {
    matchMedia(false);
    const updateHash = vi.fn();
    const scrollIntoView = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    render(
      <Harness
        updateHash={updateHash}
        id="target"
        options={{ behavior: "smooth", updateURL: true }}
      />,
    );
    act(() => {
      document.querySelector<HTMLButtonElement>('[data-testid="go"]')?.click();
    });
    expect(document.querySelector<HTMLInputElement>("#tab-b")?.checked).toBe(
      true,
    );
    expect(scrollIntoView).toHaveBeenCalledWith({
      block: "start",
      behavior: "smooth",
    });
    expect(updateHash).toHaveBeenCalledWith("target");
  });

  it("lands on the matched panel when its heading is outside that tab", () => {
    const updateHash = vi.fn();
    const scroll = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    render(
      <Harness updateHash={updateHash} id="" options={{ tabId: "tab-b" }} />,
    );
    act(() => {
      document.querySelector<HTMLButtonElement>('[data-testid="go"]')?.click();
    });
    expect(document.querySelector<HTMLInputElement>("#tab-b")?.checked).toBe(
      true,
    );
    expect(scroll.mock.instances.at(-1)).toBe(
      document.querySelector('[data-m2h-tab-id="tab-b"]'),
    );
    expect(updateHash).toHaveBeenCalledWith(null);
  });

  it("leaves the URL untouched when updateURL is false", () => {
    const updateHash = vi.fn();
    vi.spyOn(Element.prototype, "scrollIntoView").mockImplementation(() => {});
    render(
      <Harness
        updateHash={updateHash}
        id="target"
        options={{ updateURL: false }}
      />,
    );
    act(() => {
      document.querySelector<HTMLButtonElement>('[data-testid="go"]')?.click();
    });
    expect(updateHash).not.toHaveBeenCalled();
  });

  it("collapses to instant scrolling under prefers-reduced-motion", () => {
    matchMedia(true);
    const updateHash = vi.fn();
    const scrollIntoView = vi
      .spyOn(Element.prototype, "scrollIntoView")
      .mockImplementation(() => {});
    render(
      <Harness
        updateHash={updateHash}
        id="target"
        options={{ behavior: "smooth" }}
      />,
    );
    act(() => {
      document.querySelector<HTMLButtonElement>('[data-testid="go"]')?.click();
    });
    expect(scrollIntoView).toHaveBeenCalledWith(
      expect.objectContaining({ behavior: "auto" }),
    );
  });
});
