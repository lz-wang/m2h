import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { LightboxImage } from "../lib/image-lightbox";
import { ImageLightbox } from "./image-lightbox";

// jsdom runs no layout: the ResizeObserver stub never reports sizes, so the
// component's geometry stays unknown and the transform math falls back to its
// neutral baseline (fitScale 1, free pan). The zoom sequence below is exact in
// binary floating point (powers of 5/4), so the transform-string assertions
// are stable.
function makeImages(count: number): LightboxImage[] {
  return Array.from({ length: count }, (_, index) => ({
    src: `/img-${index}.png`,
    srcSet: null,
    sizes: null,
    alt: `Image ${index}`,
    title: null,
  }));
}

function renderLightbox(
  images: LightboxImage[],
  index: number,
  handlers?: { onIndexChange?: (index: number) => void; onClose?: () => void },
) {
  const onIndexChange = handlers?.onIndexChange ?? vi.fn();
  const onClose = handlers?.onClose ?? vi.fn();
  render(
    <ImageLightbox
      images={images}
      index={index}
      onIndexChange={onIndexChange}
      onClose={onClose}
    />,
  );
  return { onIndexChange, onClose };
}

function currentImage(): HTMLImageElement {
  const dialog = screen.getByRole("dialog");
  const image = dialog.querySelector<HTMLImageElement>("img");
  if (image === null) {
    throw new Error("lightbox image was not rendered");
  }
  return image;
}

describe("ImageLightbox", () => {
  it("shows the image at the given index", () => {
    const images = makeImages(3);
    renderLightbox(images, 1);

    expect(currentImage().getAttribute("src")).toBe("/img-1.png");
    expect(screen.getByText("2 / 3")).toBeTruthy();
    // An accessible name for the dialog and the position for screen readers.
    expect(screen.getByRole("dialog", { name: "图片预览" })).toBeTruthy();
    expect(screen.getByText("第 2 张，共 3 张")).toBeTruthy();
  });

  it("navigates to the previous and next image through the toolbar", async () => {
    const images = makeImages(3);
    const { onIndexChange } = renderLightbox(images, 1);

    await userEvent.click(screen.getByRole("button", { name: "上一张图片" }));
    expect(onIndexChange).toHaveBeenCalledWith(0);

    await userEvent.click(screen.getByRole("button", { name: "下一张图片" }));
    expect(onIndexChange).toHaveBeenCalledWith(2);
  });

  it("navigates with the arrow keys", () => {
    const images = makeImages(3);
    const { onIndexChange } = renderLightbox(images, 1);

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "ArrowLeft" });
    expect(onIndexChange).toHaveBeenCalledWith(0);

    fireEvent.keyDown(screen.getByRole("dialog"), { key: "ArrowRight" });
    expect(onIndexChange).toHaveBeenCalledWith(2);
  });

  it("disables previous on the first image and next on the last", () => {
    const images = makeImages(3);
    renderLightbox(images, 0);
    expect(
      (screen.getByRole("button", { name: "上一张图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
    expect(
      (screen.getByRole("button", { name: "下一张图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(false);

    renderLightbox(images, 2);
    expect(
      (screen.getByRole("button", { name: "上一张图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(false);
    expect(
      (screen.getByRole("button", { name: "下一张图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("zooms in to the cap and back out to the floor", async () => {
    renderLightbox(makeImages(1), 0);

    const zoomIn = screen.getByRole("button", { name: "放大图片" });
    await userEvent.click(zoomIn);
    expect(currentImage().style.transform).toContain("scale(1.25)");
    await userEvent.click(zoomIn);
    expect(currentImage().style.transform).toContain("scale(1.5625)");

    for (let click = 0; click < 12; click += 1) {
      await userEvent.click(zoomIn);
    }
    const peaked = currentImage().style.transform;
    expect(peaked).toContain("scale(5)");
    expect(
      (screen.getByRole("button", { name: "放大图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);

    const zoomOut = screen.getByRole("button", { name: "缩小图片" });
    for (let click = 0; click < 12; click += 1) {
      await userEvent.click(zoomOut);
    }
    expect(currentImage().style.transform).toContain("scale(1)");
    expect(
      (screen.getByRole("button", { name: "缩小图片" }) as HTMLButtonElement)
        .disabled,
    ).toBe(true);
  });

  it("rotates in quarter turns in both directions", async () => {
    renderLightbox(makeImages(1), 0);

    const clockwise = screen.getByRole("button", { name: "顺时针旋转" });
    await userEvent.click(clockwise);
    expect(currentImage().style.transform).toContain("rotate(90deg)");
    await userEvent.click(clockwise);
    expect(currentImage().style.transform).toContain("rotate(180deg)");
    await userEvent.click(clockwise);
    expect(currentImage().style.transform).toContain("rotate(270deg)");
    await userEvent.click(clockwise);
    expect(currentImage().style.transform).toContain("rotate(0deg)");

    const counterClockwise = screen.getByRole("button", { name: "逆时针旋转" });
    await userEvent.click(counterClockwise);
    expect(currentImage().style.transform).toContain("rotate(270deg)");
    await userEvent.click(counterClockwise);
    expect(currentImage().style.transform).toContain("rotate(180deg)");
  });

  it("pans the image with pointer drags", () => {
    renderLightbox(makeImages(1), 0);

    const image = currentImage();
    fireEvent.pointerDown(image, { pointerId: 1, clientX: 100, clientY: 80 });
    fireEvent.pointerMove(image, { pointerId: 1, clientX: 200, clientY: 130 });
    fireEvent.pointerUp(image, { pointerId: 1, clientX: 200, clientY: 130 });

    expect(image.style.transform).toContain("translate3d(100px, 50px, 0)");
    expect(image.dataset.panning).toBeUndefined();
  });

  it("ignores pointer moves from other pointers during a drag", () => {
    renderLightbox(makeImages(1), 0);

    const image = currentImage();
    fireEvent.pointerDown(image, { pointerId: 1, clientX: 0, clientY: 0 });
    fireEvent.pointerMove(image, { pointerId: 2, clientX: 500, clientY: 500 });
    fireEvent.pointerUp(image, { pointerId: 1, clientX: 0, clientY: 0 });

    expect(image.style.transform).toContain("translate3d(0px, 0px, 0)");
  });

  it("resets zoom, rotation, and pan when the image changes", async () => {
    const images = makeImages(2);
    const { onIndexChange } = renderLightbox(images, 0);

    // Build up non-default viewing state on the first image.
    const zoomIn = screen.getByRole("button", { name: "放大图片" });
    await userEvent.click(zoomIn);
    await userEvent.click(zoomIn);
    await userEvent.click(screen.getByRole("button", { name: "顺时针旋转" }));
    const image = currentImage();
    fireEvent.pointerDown(image, { pointerId: 1, clientX: 0, clientY: 0 });
    fireEvent.pointerMove(image, { pointerId: 1, clientX: 100, clientY: 50 });
    fireEvent.pointerUp(image, { pointerId: 1, clientX: 100, clientY: 50 });
    expect(image.style.transform).toContain("translate3d(100px, 50px, 0)");
    expect(image.style.transform).toContain("rotate(90deg)");

    // Switching reports the new index; the parent then re-renders with it.
    await userEvent.click(screen.getByRole("button", { name: "下一张图片" }));
    expect(onIndexChange).toHaveBeenCalledWith(1);
  });

  it("closes through the close button, blank area, and Escape", async () => {
    // Close button.
    const first = renderLightbox(makeImages(2), 0);
    await userEvent.click(screen.getByRole("button", { name: "关闭图片预览" }));
    expect(first.onClose).toHaveBeenCalledTimes(1);

    // Blank popup area (the press target is the popup itself, not the image,
    // toolbar, or close button).
    const second = renderLightbox(makeImages(2), 0);
    fireEvent.pointerDown(screen.getByRole("dialog"));
    expect(second.onClose).toHaveBeenCalledTimes(1);

    // Escape goes through the Dialog's own close handling (floating-ui listens
    // with focus inside the popup, so a toolbar button is focused first).
    const third = renderLightbox(makeImages(2), 0);
    await userEvent.click(screen.getByRole("button", { name: "放大图片" }));
    await userEvent.keyboard("{Escape}");
    expect(third.onClose).toHaveBeenCalledTimes(1);
  });

  it("does not close on a press over the image or the toolbar", () => {
    const { onClose } = renderLightbox(makeImages(2), 0);

    fireEvent.pointerDown(currentImage());
    fireEvent.pointerDown(screen.getByRole("button", { name: "放大图片" }));

    expect(onClose).not.toHaveBeenCalled();
  });

  it("renders nothing for an out-of-range index", () => {
    renderLightbox(makeImages(1), 5);
    expect(screen.queryByRole("dialog")).toBeNull();
  });
});
