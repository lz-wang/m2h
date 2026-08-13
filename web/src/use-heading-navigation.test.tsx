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
        <h2 id="target">Target</h2>
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
    expect(scrollIntoView).toHaveBeenCalledWith({
      block: "start",
      behavior: "smooth",
    });
    expect(updateHash).toHaveBeenCalledWith("target");
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
