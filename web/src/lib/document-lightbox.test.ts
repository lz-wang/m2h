import { describe, expect, it } from "vitest";

import { collectLightboxState, type LightboxItem } from "./document-lightbox";

// Two enhanced images plus one plain image, in document order.
function enhancedRoot(): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = `
    <p><img src="/one.png" alt="One" data-m2h-lightbox-item="true"></p>
    <p><img src="/two.png" alt="Two" data-m2h-lightbox-item="true"></p>
    <p><img src="/plain.png" alt="Plain"></p>
  `;
  return root;
}

// An image, a rendered Mermaid diagram (marker on the container, SVG inside)
// and another image: the mixed visual sequence the lightbox browses.
function mixedRoot(): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = `
    <p><img src="/a.png" alt="A" data-m2h-lightbox-item="true"></p>
    <div class="m2h-mermaid-frame">
      <div class="mermaid" data-m2h-lightbox-item="true">
        <svg viewBox="0 0 800 400" width="100%"><path d="M0 0"></path></svg>
      </div>
      <button type="button" class="m2h-lightbox-trigger"></button>
    </div>
    <p><img src="/c.png" alt="C" data-m2h-lightbox-item="true"></p>
  `;
  return root;
}

// The snapshot src is the browser-resolved URL (currentSrc || src), so tests
// compare paths rather than raw attribute strings.
function srcPaths(items: LightboxItem[]): string[] {
  return items.map((item) =>
    item.src.startsWith("data:") ? "mermaid" : new URL(item.src).pathname,
  );
}

describe("collectLightboxState", () => {
  it("snapshots enhanced images in DOM order and indexes the selected one", () => {
    const root = enhancedRoot();
    const selected = root.querySelectorAll<HTMLImageElement>("img")[1];

    const state = collectLightboxState(root, selected);

    expect(state).not.toBeNull();
    expect(state?.index).toBe(1);
    expect(state?.items).toHaveLength(2);
    expect(srcPaths(state?.items ?? [])).toEqual(["/one.png", "/two.png"]);
    expect(state?.items[1]?.kind).toBe("image");
    expect(state?.items[1]?.src).toContain("/two.png");
    expect(state?.items[1]?.srcSet).toBeNull();
    expect(state?.items[1]?.sizes).toBeNull();
    expect(state?.items[1]?.alt).toBe("Two");
    expect(state?.items[1]?.title).toBeNull();
  });

  it("interleaves images and mermaid diagrams in document order", () => {
    const root = mixedRoot();
    const selected = root.querySelector<HTMLElement>("div.mermaid");
    if (selected === null) {
      throw new Error("mermaid container was not created");
    }

    const state = collectLightboxState(root, selected);

    expect(state).not.toBeNull();
    expect(state?.index).toBe(1);
    expect(srcPaths(state?.items ?? [])).toEqual([
      "/a.png",
      "mermaid",
      "/c.png",
    ]);
    // The kinds follow the sources: bitmap, diagram, bitmap.
    expect(state?.items.map((item) => item.kind)).toEqual([
      "image",
      "mermaid",
      "image",
    ]);
    // The diagram snapshot is a serialized SVG data URL with the viewBox's
    // pixel dimensions pinned onto the clone — percentages have no intrinsic
    // size once the SVG stands alone as an <img>.
    const snapshot = state?.items[1];
    expect(snapshot?.src).toMatch(/^data:image\/svg\+xml;charset=utf-8,/);
    expect(snapshot?.src).toContain(encodeURIComponent('width="800"'));
    expect(snapshot?.src).toContain(encodeURIComponent('height="400"'));
    expect(snapshot?.srcSet).toBeNull();
    expect(snapshot?.sizes).toBeNull();
    expect(snapshot?.alt).toBe("Mermaid 图表");
    expect(snapshot?.title).toBeNull();
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
    expect(state?.items[0]?.src).toContain("/two.png");
    expect(state?.items[1]?.src).toContain("/one.png");
  });

  it("skips mermaid containers whose diagram never rendered", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <p><img src="/a.png" alt="A" data-m2h-lightbox-item="true"></p>
      <div class="m2h-mermaid-frame">
        <div class="mermaid" data-m2h-lightbox-item="true">graph TD</div>
      </div>
      <p><img src="/c.png" alt="C" data-m2h-lightbox-item="true"></p>
    `;
    const selected = root.querySelector("img");

    // The failed diagram is skipped by the collection, so the items still
    // number 1–2 with no hole.
    const state =
      selected === null ? null : collectLightboxState(root, selected);
    expect(srcPaths(state?.items ?? [])).toEqual(["/a.png", "/c.png"]);
    expect(state?.index).toBe(0);
  });

  it("returns null for a mermaid container whose diagram never rendered", () => {
    const root = document.createElement("div");
    root.innerHTML =
      '<div class="mermaid" data-m2h-lightbox-item="true">graph TD</div>';
    const selected = root.querySelector<HTMLElement>("div.mermaid");

    // Pressing the trigger of an unrendered diagram opens nothing.
    expect(
      selected === null ? "missing" : collectLightboxState(root, selected),
    ).toBeNull();
  });

  it("returns null for an item outside the root", () => {
    const root = enhancedRoot();
    const outsider = document.createElement("img");
    outsider.dataset.m2hLightboxItem = "true";

    expect(collectLightboxState(root, outsider)).toBeNull();
  });

  it("returns null for an image without the lightbox marker", () => {
    const root = enhancedRoot();
    const plain = root.querySelectorAll<HTMLImageElement>("img")[2];

    expect(collectLightboxState(root, plain)).toBeNull();
  });
});
