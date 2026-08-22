import { describe, expect, it } from "vitest";

import { collectLightboxState, type LightboxImage } from "./image-lightbox";

// Two enhanced images plus one plain image, in document order.
function enhancedRoot(): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = `
    <p><img src="/one.png" alt="One" data-m2h-lightbox-image="true"></p>
    <p><img src="/two.png" alt="Two" data-m2h-lightbox-image="true"></p>
    <p><img src="/plain.png" alt="Plain"></p>
  `;
  return root;
}

// The snapshot src is the browser-resolved URL (currentSrc || src), so tests
// compare paths rather than raw attribute strings.
function srcPaths(images: LightboxImage[]): string[] {
  return images.map((image) => new URL(image.src).pathname);
}

describe("collectLightboxState", () => {
  it("snapshots enhanced images in DOM order and indexes the selected one", () => {
    const root = enhancedRoot();
    const selected = root.querySelectorAll<HTMLImageElement>("img")[1];

    const state = collectLightboxState(root, selected);

    expect(state).not.toBeNull();
    expect(state?.index).toBe(1);
    expect(state?.images).toHaveLength(2);
    expect(srcPaths(state?.images ?? [])).toEqual(["/one.png", "/two.png"]);
    expect(state?.images[1]?.src).toContain("/two.png");
    expect(state?.images[1]?.srcSet).toBeNull();
    expect(state?.images[1]?.sizes).toBeNull();
    expect(state?.images[1]?.alt).toBe("Two");
    expect(state?.images[1]?.title).toBeNull();
  });

  it("re-indexes against the current DOM order, not an assigned position", () => {
    const root = enhancedRoot();
    // A sortable table moves rows: the paragraph that was second — and its
    // image — becomes first, with no marker on the DOM being rewritten.
    const paragraphs = root.querySelectorAll("p");
    root.insertBefore(paragraphs[1], paragraphs[0]);

    const moved = root.querySelectorAll<HTMLImageElement>("img")[0];
    const state = collectLightboxState(root, moved);

    expect(state?.index).toBe(0);
    expect(state?.images[0]?.src).toContain("/two.png");
    expect(state?.images[1]?.src).toContain("/one.png");
  });

  it("returns null for an image outside the root", () => {
    const root = enhancedRoot();
    const outsider = document.createElement("img");
    outsider.dataset.m2hLightboxImage = "true";

    expect(collectLightboxState(root, outsider)).toBeNull();
  });

  it("returns null for an image without the lightbox marker", () => {
    const root = enhancedRoot();
    const plain = root.querySelectorAll<HTMLImageElement>("img")[2];

    expect(collectLightboxState(root, plain)).toBeNull();
  });
});
