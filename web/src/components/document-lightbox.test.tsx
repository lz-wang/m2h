import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
import { describe, expect, it, vi } from "vitest";

import type { LightboxItem } from "../lib/document-lightbox";
import { DocumentLightbox } from "./document-lightbox";

// jsdom runs no layout: the ResizeObserver stub never reports sizes, so the
// component's geometry stays unknown and the transform math falls back to its
// neutral baseline (fitScale 1, free pan). The zoom sequence below is exact in
// binary floating point (powers of 5/4), so the transform-string assertions
// are stable.
function makeItems(count: number): LightboxItem[] {
  return Array.from({ length: count }, (_, index) => ({
    kind: "image" as const,
    src: `/img-${index}.png`,
    srcSet: null,
    sizes: null,
    alt: `Image ${index}`,
    title: null,
  }));
}

function renderLightbox(
  items: LightboxItem[],
  index: number,
  handlers?: { onIndexChange?: (index: number) => void; onClose?: () => void },
) {
  const onIndexChange = handlers?.onIndexChange ?? vi.fn();
  const onClose = handlers?.onClose ?? vi.fn();
  render(
    <DocumentLightbox
      items={items}
      index={index}
      open
      onIndexChange={onIndexChange}
      onClose={onClose}
      onClosed={vi.fn()}
    />,
  );
  return { onIndexChange, onClose };
}

// The parent's actual shape: closing flips `open`, and the snapshot state is
// dropped only when the dialog reports its exit transition finished.
function ControlledLightbox({
  items,
  index,
  onClose,
  onClosed,
}: {
  items: LightboxItem[];
  index: number;
  onClose(): void;
  onClosed(): void;
}) {
  const [open, setOpen] = useState(true);
  return (
    <DocumentLightbox
      items={items}
      index={index}
      open={open}
      onIndexChange={() => {}}
      onClose={() => {
        onClose();
        setOpen(false);
      }}
      onClosed={onClosed}
    />
  );
}

function currentItem(): HTMLImageElement {
  const dialog = screen.getByRole("dialog");
  const image = dialog.querySelector<HTMLImageElement>("img");
  if (image === null) {
    throw new Error("lightbox image was not rendered");
  }
  return image;
}

function currentScale(image: HTMLImageElement): number {
  const match = image.style.transform.match(/scale\(([^)]+)\)/);
  if (match === null) {
    throw new Error("lightbox image scale was not rendered");
  }
  return Number(match[1]);
}

describe("DocumentLightbox", () => {
  it("shows the item at the given index", () => {
    const items = makeItems(3);
    renderLightbox(items, 1);

    expect(currentItem().getAttribute("src")).toBe("/img-1.png");
    expect(screen.getByText("2 / 3")).toBeTruthy();
    // An accessible name for the dialog and the position for screen readers.
    expect(screen.getByRole("dialog", { name: "视觉内容预览" })).toBeTruthy();
    expect(screen.getByText("第 2 项，共 3 项")).toBeTruthy();
  });

  it("navigates to the previous and next item through the toolbar", async () => {
    const items = makeItems(3);
    const { onIndexChange } = renderLightbox(items, 1);

    await userEvent.click(screen.getByRole("button", { name: "上一项" }));
    expect(onIndexChange).toHaveBeenCalledWith(0);

    await userEvent.click(screen.getByRole("button", { name: "下一项" }));
    expect(onIndexChange).toHaveBeenCalledWith(2);
  });

  it("navigates with the arrow keys", () => {
    const items = makeItems(3);
    const { onIndexChange } = renderLightbox(items, 1);

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "ArrowLeft" });
    expect(onIndexChange).toHaveBeenCalledWith(0);

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "ArrowRight" });
    expect(onIndexChange).toHaveBeenCalledWith(2);
  });

  it("disables previous on the first item and next on the last", () => {
    const items = makeItems(3);
    renderLightbox(items, 0);
    expect(
      (screen.getByRole("button", { name: "上一项" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    expect(
      (screen.getByRole("button", { name: "下一项" }) as HTMLButtonElement)
        .disabled,
    ).toBe(false);

    renderLightbox(items, 2);
    expect(
      (screen.getByRole("button", { name: "上一项" }) as HTMLButtonElement)
        .disabled,
    ).toBe(false);
    expect(
      (screen.getByRole("button", { name: "下一项" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("zooms in to the cap and back out to the floor", async () => {
    renderLightbox(makeItems(1), 0);

    const zoomIn = screen.getByRole("button", { name: "放大图片" });
    await userEvent.click(zoomIn);
    expect(currentItem().style.transform).toContain("scale(1.25)");
    await userEvent.click(zoomIn);
    expect(currentItem().style.transform).toContain("scale(1.5625)");

    for (let click = 0; click < 12; click += 1) {
      await userEvent.click(zoomIn);
    }
    const peaked = currentItem().style.transform;
    expect(peaked).toContain("scale(5)");
    expect(
      (screen.getByRole("button", { name: "放大图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    const zoomOut = screen.getByRole("button", { name: "缩小图片" });
    for (let click = 0; click < 12; click += 1) {
      await userEvent.click(zoomOut);
    }
    expect(currentItem().style.transform).toContain("scale(1)");
    expect(
      (screen.getByRole("button", { name: "缩小图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it.each([
    ["图片", makeItems(1)[0]],
    [
      "Mermaid 图表",
      {
        kind: "mermaid" as const,
        src: "data:image/svg+xml,%3Csvg%3E%3C/svg%3E",
        srcSet: null,
        sizes: null,
        alt: "Mermaid 图表",
        title: null,
      },
    ],
  ])("uses the mouse wheel to zoom the %s", (_label, item) => {
    renderLightbox([item], 0);

    const image = currentItem();
    const zoomInEvent = new WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      deltaY: -10,
    });
    fireEvent(image, zoomInEvent);

    expect(zoomInEvent.defaultPrevented).toBe(true);
    expect(currentScale(image)).toBeCloseTo(Math.exp(0.008), 5);
    expect(currentScale(image)).toBeLessThan(1.01);

    const zoomOutEvent = new WheelEvent("wheel", {
      bubbles: true,
      cancelable: true,
      deltaY: 10,
    });
    fireEvent(image, zoomOutEvent);

    expect(zoomOutEvent.defaultPrevented).toBe(true);
    expect(currentScale(image)).toBeCloseTo(1, 5);
  });

  it("caps one large wheel event below a ten-percent zoom jump", () => {
    renderLightbox(makeItems(1), 0);

    const image = currentItem();
    fireEvent.wheel(image, { deltaY: -1000 });

    expect(currentScale(image)).toBeCloseTo(Math.exp(0.08), 5);
    expect(currentScale(image)).toBeLessThan(1.1);
  });

  it("rotates in quarter turns in both directions", async () => {
    renderLightbox(makeItems(1), 0);

    const clockwise = screen.getByRole("button", { name: "顺时针旋转" });
    await userEvent.click(clockwise);
    expect(currentItem().style.transform).toContain("rotate(90deg)");
    await userEvent.click(clockwise);
    expect(currentItem().style.transform).toContain("rotate(180deg)");
    await userEvent.click(clockwise);
    expect(currentItem().style.transform).toContain("rotate(270deg)");
    await userEvent.click(clockwise);
    expect(currentItem().style.transform).toContain("rotate(0deg)");

    const counterClockwise = screen.getByRole("button", { name: "逆时针旋转" });
    await userEvent.click(counterClockwise);
    expect(currentItem().style.transform).toContain("rotate(270deg)");
    await userEvent.click(counterClockwise);
    expect(currentItem().style.transform).toContain("rotate(180deg)");
  });

  it("pans the image with pointer drags", () => {
    renderLightbox(makeItems(1), 0);

    const image = currentItem();
    fireEvent.pointerDown(image, { pointerId: 1, clientX: 100, clientY: 80 });
    fireEvent.pointerMove(image, { pointerId: 1, clientX: 200, clientY: 130 });
    fireEvent.pointerUp(image, { pointerId: 1, clientX: 200, clientY: 130 });

    expect(image.style.transform).toContain("translate3d(100px, 50px, 0)");
    expect(image.dataset.panning).toBeUndefined();
  });

  it("ignores pointer moves from other pointers during a drag", () => {
    renderLightbox(makeItems(1), 0);

    const image = currentItem();
    fireEvent.pointerDown(image, { pointerId: 1, clientX: 0, clientY: 0 });
    fireEvent.pointerMove(image, { pointerId: 2, clientX: 500, clientY: 500 });
    fireEvent.pointerUp(image, { pointerId: 1, clientX: 0, clientY: 0 });

    expect(image.style.transform).toContain("translate3d(0px, 0px, 0)");
  });

  it("resets zoom, rotation, and pan when the item changes", async () => {
    const items = makeItems(2);
    const onIndexChange = vi.fn();
    const onClose = vi.fn();
    const view = render(
      <DocumentLightbox
        items={items}
        index={0}
        open
        onIndexChange={onIndexChange}
        onClose={onClose}
        onClosed={vi.fn()}
      />,
    );

    // Build up non-default viewing state on the first item, ending mid-drag.
    const zoomIn = screen.getByRole("button", { name: "放大图片" });
    await userEvent.click(zoomIn);
    await userEvent.click(zoomIn);
    await userEvent.click(screen.getByRole("button", { name: "顺时针旋转" }));
    const image = currentItem();
    fireEvent.pointerDown(image, { pointerId: 1, clientX: 0, clientY: 0 });
    fireEvent.pointerMove(image, { pointerId: 1, clientX: 100, clientY: 50 });
    expect(image.style.transform).toContain("translate3d(100px, 50px, 0)");
    expect(image.style.transform).toContain("rotate(90deg)");
    expect(image.style.transform).toContain("scale(1.5625)");
    expect(image.dataset.panning).toBe("true");

    // The parent feeds the switched index back as a rerender, exactly like
    // App.tsx does — the reset must land in that same commit, before paint.
    view.rerender(
      <DocumentLightbox
        items={items}
        index={1}
        open
        onIndexChange={onIndexChange}
        onClose={onClose}
        onClosed={vi.fn()}
      />,
    );

    const nextItem = currentItem();
    expect(nextItem.getAttribute("src")).toBe("/img-1.png");
    expect(nextItem.style.transform).toContain("translate3d(0px, 0px, 0)");
    expect(nextItem.style.transform).toContain("rotate(0deg)");
    expect(nextItem.style.transform).toContain("scale(1)");
    // The in-flight drag died with the switch: no panning flag, and later
    // moves from the same pointer land nowhere.
    expect(nextItem.dataset.panning).toBeUndefined();
    fireEvent.pointerMove(nextItem, {
      pointerId: 1,
      clientX: 500,
      clientY: 500,
    });
    expect(nextItem.style.transform).toContain("translate3d(0px, 0px, 0)");
  });

  it("closes through the close button, blank area, and Escape", async () => {
    // Close button.
    const first = renderLightbox(makeItems(2), 0);
    await userEvent.click(
      screen.getByRole("button", { name: "关闭视觉内容预览" }),
    );
    expect(first.onClose).toHaveBeenCalledTimes(1);

    // Blank popup area (the press target is the popup itself, not the image,
    // toolbar, or close button).
    const second = renderLightbox(makeItems(2), 0);
    fireEvent.pointerDown(screen.getByRole("dialog"));
    expect(second.onClose).toHaveBeenCalledTimes(1);

    // Escape goes through the Dialog's own close handling (floating-ui listens
    // with focus inside the popup, so a toolbar button is focused first).
    const third = renderLightbox(makeItems(2), 0);
    await userEvent.click(screen.getByRole("button", { name: "放大图片" }));
    await userEvent.keyboard("{Escape}");
    expect(third.onClose).toHaveBeenCalledTimes(1);
  });

  it("reports the finished exit transition so the parent can drop the state", async () => {
    const onClose = vi.fn();
    const onClosed = vi.fn();
    render(
      <ControlledLightbox
        items={makeItems(2)}
        index={0}
        onClose={onClose}
        onClosed={onClosed}
      />,
    );

    await userEvent.click(
      screen.getByRole("button", { name: "关闭视觉内容预览" }),
    );
    expect(onClose).toHaveBeenCalledTimes(1);

    // jsdom computes no transitions, so the completion fires promptly; in a
    // real browser this is what waits out the 160/180ms exit animation.
    await waitFor(() => expect(onClosed).toHaveBeenCalledTimes(1));
  });

  it("does not close on a press over the image or the toolbar", () => {
    const { onClose } = renderLightbox(makeItems(2), 0);

    fireEvent.pointerDown(currentItem());
    fireEvent.pointerDown(screen.getByRole("button", { name: "放大图片" }));

    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing for an out-of-range index", () => {
    renderLightbox(makeItems(1), 5);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
