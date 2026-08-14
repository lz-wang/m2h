import { describe, expect, it, vi } from "vitest";
import { restoreScrollTopWhenStable } from "./scroll-restoration";

// jsdom has no layout engine, so scrollHeight is a constant 0. Feed the
// stability loop explicitly instead: the heights array is read once per tick,
// and a test appends a new value to simulate the body still reflowing.
function createViewport(heights: number[]): {
  viewport: HTMLElement;
  scrollTopWrites: number[];
} {
  const viewport = document.createElement("div");
  const scrollTopWrites: number[] = [];
  Object.defineProperty(viewport, "scrollHeight", {
    configurable: true,
    get: () => heights[0],
  });
  Object.defineProperty(viewport, "scrollTop", {
    configurable: true,
    get: () => scrollTopWrites[scrollTopWrites.length - 1] ?? 0,
    set: (value: number) => {
      scrollTopWrites.push(value);
    },
  });
  return { viewport, scrollTopWrites };
}

// The test setup maps each rAF to one setTimeout(0) macrotask, so awaiting one
// timer lets exactly one frame callback run.
async function frames(count: number): Promise<void> {
  for (let i = 0; i < count; i += 1) {
    await new Promise((resolve) => {
      setTimeout(resolve, 0);
    });
  }
}

describe("restoreScrollTopWhenStable", () => {
  it("re-applies the target every frame and settles once the height is stable", async () => {
    const heights = [1000];
    const { viewport, scrollTopWrites } = createViewport(heights);
    const onSettled = vi.fn();
    const previousAnchor = viewport.style.overflowAnchor;

    restoreScrollTopWhenStable(viewport, 4287, onSettled);

    // First frame runs before the height is first observed: the offset is
    // already applied and scroll anchoring is suppressed for the interval.
    await frames(1);
    expect(scrollTopWrites).toEqual([4287]);
    expect(viewport.style.overflowAnchor).not.toBe(previousAnchor);

    // The body keeps reflowing for two more frames; the offset is re-applied
    // each time and the loop keeps waiting.
    heights[0] = 1200;
    await frames(2);
    expect(scrollTopWrites.length).toBe(3);
    expect(onSettled).not.toHaveBeenCalled();

    // Three consecutive identical heights settle the loop: the target is
    // written once more, scroll anchoring is restored, and onSettled fires.
    await frames(3);
    expect(onSettled).toHaveBeenCalledTimes(1);
    expect(scrollTopWrites.every((value) => value === 4287)).toBe(true);
    expect(viewport.style.overflowAnchor).toBe(previousAnchor);
  });

  it("gives up after the frame cap even when the layout never settles", async () => {
    const heights = [1];
    const { viewport, scrollTopWrites } = createViewport(heights);
    const onSettled = vi.fn();
    let heightReads = 0;
    Object.defineProperty(viewport, "scrollHeight", {
      configurable: true,
      get: () => {
        heightReads += 1;
        return heightReads;
      },
    });

    restoreScrollTopWhenStable(viewport, 2500, onSettled);

    await frames(30);
    expect(onSettled).toHaveBeenCalledTimes(1);
    // 30 frame writes plus the final write inside finish().
    expect(scrollTopWrites.length).toBe(31);
    // The cap is a floor, not a ceiling for further frames: the loop stops
    // scheduling once maxFrames is reached.
    await frames(3);
    expect(scrollTopWrites.length).toBe(31);
    expect(onSettled).toHaveBeenCalledTimes(1);
  });

  it("cancel stops the loop without writing the final offset or settling", async () => {
    const heights = [1000];
    const { viewport, scrollTopWrites } = createViewport(heights);
    const onSettled = vi.fn();
    const previousAnchor = viewport.style.overflowAnchor;

    const cancel = restoreScrollTopWhenStable(viewport, 4287, onSettled);
    await frames(2);
    cancel();
    const writesAfterCancel = scrollTopWrites.length;

    await frames(6);
    expect(scrollTopWrites.length).toBe(writesAfterCancel);
    expect(onSettled).not.toHaveBeenCalled();
    expect(viewport.style.overflowAnchor).toBe(previousAnchor);
  });
});
