import { describe, expect, it } from "vitest";

import { collectLightboxState, type LightboxItem } from "./document-lightbox";

function enhancedRoot(): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = `
    <p><img src="/one.png" alt="One" data-m2h-lightbox-item="true"></p>
    <p><img src="/two.png" alt="Two" data-m2h-lightbox-item="true"></p>
    <p><img src="/plain.png" alt="Plain"></p>
  `;
  return root;
}

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
    <div class="m2h-vega-lite" data-m2h-lightbox-item="true">
      <svg viewBox="0 0 600 300"><rect width="600" height="300"></rect></svg>
    </div>
    <p><img src="/c.png" alt="C" data-m2h-lightbox-item="true"></p>
  `;
  return root;
}

function itemPaths(items: LightboxItem[]): string[] {
  return items.map((item) =>
    item.kind === "image" ? new URL(item.src).pathname : item.kind,
  );
}

describe("collectLightboxState", () => {
  it("namespaces SVG identifiers and their local references in Lightbox snapshots", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <div class="mermaid" data-m2h-lightbox-item="true">
        <svg viewBox="0 0 100 50">
          <defs>
            <linearGradient id="gradient"><stop offset="0"></stop></linearGradient>
            <clipPath id="clip"><path id="shape" d="M0 0"></path></clipPath>
          </defs>
          <style>.mark { fill: url(#gradient); }</style>
          <title id="title">图表</title>
          <g aria-labelledby="title" clip-path="url(#clip)">
            <use href="#shape" style="fill: url('#gradient')"></use>
          </g>
        </svg>
      </div>
    `;
    const selected = root.querySelector<HTMLElement>(".mermaid");
    if (selected === null) throw new Error("Mermaid container was not created");

    const state = collectLightboxState(root, selected);
    const item = state?.items[0];
    if (item?.kind === "image" || item === undefined) {
      throw new Error("expected an SVG Lightbox item");
    }
    const snapshot = new DOMParser().parseFromString(
      item.markup,
      "image/svg+xml",
    );
    const sourceIDs = new Set(
      Array.from(
        root.querySelectorAll<HTMLElement>("svg [id]"),
        (element) => element.id,
      ),
    );
    const snapshotIDs = Array.from(
      snapshot.querySelectorAll<HTMLElement>("[id]"),
      (element) => element.id,
    );

    expect(snapshotIDs).toEqual([
      "m2h-lightbox-0-gradient",
      "m2h-lightbox-0-clip",
      "m2h-lightbox-0-shape",
      "m2h-lightbox-0-title",
    ]);
    expect(snapshotIDs.some((id) => sourceIDs.has(id))).toBe(false);
    expect(snapshot.querySelector("style")?.textContent).toContain(
      "url(#m2h-lightbox-0-gradient)",
    );
    expect(snapshot.querySelector("g")?.getAttribute("aria-labelledby")).toBe(
      "m2h-lightbox-0-title",
    );
    expect(snapshot.querySelector("g")?.getAttribute("clip-path")).toBe(
      "url(#m2h-lightbox-0-clip)",
    );
    expect(snapshot.querySelector("use")?.getAttribute("href")).toBe(
      "#m2h-lightbox-0-shape",
    );
    expect(snapshot.querySelector("use")?.getAttribute("style")).toContain(
      "url('#m2h-lightbox-0-gradient')",
    );
  });

  it("hands the snapshot's viewport geometry to the Lightbox", () => {
    // Mermaid's useMaxWidth shape: width="100%" plus an inline max-width pins
    // the diagram to its natural size and would survive any stylesheet rule,
    // so the normalization must land on the snapshot's own inline style.
    const root = document.createElement("div");
    root.innerHTML = `
      <div class="mermaid" data-m2h-lightbox-item="true">
        <svg viewBox="0 0 700 400" width="100%" style="max-width: 700px">
          <path d="M0 0"></path>
        </svg>
      </div>
    `;
    const selected = root.querySelector<HTMLElement>(".mermaid");
    if (selected === null) throw new Error("Mermaid container was not created");

    const state = collectLightboxState(root, selected);
    const item = state?.items[0];
    if (item?.kind === "image" || item === undefined) {
      throw new Error("expected an SVG Lightbox item");
    }
    const snapshot = new DOMParser().parseFromString(
      item.markup,
      "image/svg+xml",
    );
    const svg = snapshot.querySelector("svg");
    if (svg === null) throw new Error("snapshot svg was not serialized");

    // The intrinsic size stays in viewBox; the viewport belongs to the
    // Lightbox alone.
    expect(svg.getAttribute("viewBox")).toBe("0 0 700 400");
    expect(svg.hasAttribute("width")).toBe(false);
    expect(svg.hasAttribute("height")).toBe(false);
    const style = svg.getAttribute("style") ?? "";
    expect(style).toContain("width: 100% !important");
    expect(style).toContain("height: 100% !important");
    expect(style).toContain("max-width: none !important");
    expect(style).toContain("max-height: none !important");
    expect(style).toMatch(/min-width:\s*0(px)? !important/);
    expect(style).toMatch(/min-height:\s*0(px)? !important/);
    expect(style).not.toContain("max-width: 700px");
  });

  it("adds a viewBox when the source SVG carries none", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <div class="mermaid" data-m2h-lightbox-item="true">
        <svg width="400" height="200"><path d="M0 0"></path></svg>
      </div>
    `;
    const selected = root.querySelector<HTMLElement>(".mermaid");
    if (selected === null) throw new Error("Mermaid container was not created");

    const state = collectLightboxState(root, selected);
    const item = state?.items[0];
    if (item?.kind === "image" || item === undefined) {
      throw new Error("expected an SVG Lightbox item");
    }
    const snapshot = new DOMParser().parseFromString(
      item.markup,
      "image/svg+xml",
    );
    const svg = snapshot.querySelector("svg");
    // The attribute-derived intrinsic size becomes the viewBox, so a future
    // renderer emitting bare width/height still zooms as a true vector.
    expect(svg?.getAttribute("viewBox")).toBe("0 0 400 200");
    expect(svg?.hasAttribute("width")).toBe(false);
    expect(svg?.hasAttribute("height")).toBe(false);
    expect(item.intrinsicWidth).toBe(400);
    expect(item.intrinsicHeight).toBe(200);
  });

  it("keeps embedded vector links in the markup but takes them out of tab order", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <div class="m2h-vega-lite" data-m2h-lightbox-item="true">
        <svg viewBox="0 0 100 50">
          <a href="https://example.invalid/linked">
            <rect width="10" height="10"></rect>
          </a>
        </svg>
      </div>
    `;
    const selected = root.querySelector<HTMLElement>(".m2h-vega-lite");
    if (selected === null)
      throw new Error("Vega-Lite container was not created");

    const state = collectLightboxState(root, selected);
    const item = state?.items[0];
    if (item?.kind === "image" || item === undefined) {
      throw new Error("expected an SVG Lightbox item");
    }
    const snapshot = new DOMParser().parseFromString(
      item.markup,
      "image/svg+xml",
    );
    const anchor = snapshot.querySelector("a");
    // The link survives visually intact for hit-testing and text selection…
    expect(anchor?.getAttribute("href")).toBe("https://example.invalid/linked");
    // …but keyboard focus must never land inside the Lightbox snapshot.
    expect(anchor?.getAttribute("tabindex")).toBe("-1");
  });

  it("rewrites the renderer's root-id CSS scope inside embedded styles", () => {
    // Mermaid scopes its palette to the root SVG id it rendered with; after
    // the root is namespaced the selectors must follow, or the Lightbox copy
    // silently loses its styles.
    const root = document.createElement("div");
    root.innerHTML = `
      <div class="mermaid" data-m2h-lightbox-item="true">
        <svg id="m2h-mermaid-1" viewBox="0 0 100 50">
          <style>
            #m2h-mermaid-1{font-family:"trebuchet ms",verdana;fill:#333;}
            #m2h-mermaid-1 .node rect{fill:red;}
            #m2h-mermaid-1-suffix .edge{stroke:blue;}
            .plain { fill: url(#gradient); }
          </style>
          <defs>
            <linearGradient id="gradient"><stop offset="0"></stop></linearGradient>
            <g id="m2h-mermaid-1-suffix"></g>
          </defs>
          <g class="node"><rect fill="url(#gradient)"></rect></g>
        </svg>
      </div>
    `;
    const selected = root.querySelector<HTMLElement>(".mermaid");
    if (selected === null) throw new Error("Mermaid container was not created");

    const state = collectLightboxState(root, selected);
    const item = state?.items[0];
    if (item?.kind === "image" || item === undefined) {
      throw new Error("expected an SVG Lightbox item");
    }
    const snapshot = new DOMParser().parseFromString(
      item.markup,
      "image/svg+xml",
    );
    const styleText = snapshot.querySelector("style")?.textContent ?? "";

    // Every root-id scope follows the renamed root…
    expect(styleText).toContain("#m2h-lightbox-0-m2h-mermaid-1{");
    expect(styleText).toContain("#m2h-lightbox-0-m2h-mermaid-1 .node rect");
    expect(styleText).not.toContain("#m2h-mermaid-1{");
    expect(styleText).not.toContain("#m2h-mermaid-1 .node");
    // …including the suffixed element id, without clobbering longer ids…
    expect(styleText).toContain("#m2h-lightbox-0-m2h-mermaid-1-suffix .edge");
    // …and url() references still resolve after both passes.
    expect(styleText).toContain("url(#m2h-lightbox-0-gradient)");
    expect(snapshot.querySelector("svg")?.id).toBe(
      "m2h-lightbox-0-m2h-mermaid-1",
    );
    expect(
      snapshot.querySelector("linearGradient")?.id,
    ).toBe("m2h-lightbox-0-gradient");
  });

  it("snapshots enhanced images in DOM order and indexes the selected one", () => {
    const root = enhancedRoot();
    const selected = root.querySelectorAll<HTMLImageElement>("img")[1];
    const state = collectLightboxState(root, selected);

    expect(state).not.toBeNull();
    expect(state?.index).toBe(1);
    expect(state?.items).toHaveLength(2);
    expect(itemPaths(state?.items ?? [])).toEqual(["/one.png", "/two.png"]);
    expect(state?.items[1]).toMatchObject({
      kind: "image",
      srcSet: null,
      sizes: null,
      alt: "Two",
      title: null,
    });
  });

  it("interleaves bitmap, Mermaid, and Vega-Lite snapshots in document order", () => {
    const root = mixedRoot();
    const selected = root.querySelector<HTMLElement>("div.mermaid");
    if (selected === null) throw new Error("mermaid container was not created");

    const state = collectLightboxState(root, selected);

    expect(state?.index).toBe(1);
    expect(itemPaths(state?.items ?? [])).toEqual([
      "/a.png",
      "mermaid",
      "vega-lite",
      "/c.png",
    ]);
    const mermaid = state?.items[1];
    const vegaLite = state?.items[2];
    if (mermaid?.kind === "image" || vegaLite?.kind === "image") {
      throw new Error("expected SVG visuals");
    }
    expect(mermaid).toMatchObject({
      kind: "mermaid",
      intrinsicWidth: 800,
      intrinsicHeight: 400,
      alt: "Mermaid 图表",
      title: null,
    });
    expect(vegaLite).toMatchObject({
      kind: "vega-lite",
      intrinsicWidth: 600,
      intrinsicHeight: 300,
      alt: "Vega-Lite 图表",
      title: null,
    });
    expect(mermaid?.markup).toContain("<svg");
    expect(vegaLite?.markup).toContain("<svg");
    expect(mermaid?.markup).not.toContain("data:image/svg+xml");
  });

  it("re-indexes against the current DOM order, not an assigned position", () => {
    const root = enhancedRoot();
    const paragraphs = root.querySelectorAll("p");
    root.insertBefore(paragraphs[1], paragraphs[0]);

    const moved = root.querySelectorAll<HTMLImageElement>("img")[0];
    const state = collectLightboxState(root, moved);
    expect(state?.index).toBe(0);
    expect(itemPaths(state?.items ?? [])).toEqual(["/two.png", "/one.png"]);
  });

  it("skips Mermaid containers whose diagram never rendered", () => {
    const root = document.createElement("div");
    root.innerHTML = `
      <p><img src="/a.png" alt="A" data-m2h-lightbox-item="true"></p>
      <div class="mermaid" data-m2h-lightbox-item="true">graph TD</div>
      <p><img src="/c.png" alt="C" data-m2h-lightbox-item="true"></p>
    `;
    const selected = root.querySelector("img");
    const state =
      selected === null ? null : collectLightboxState(root, selected);
    expect(itemPaths(state?.items ?? [])).toEqual(["/a.png", "/c.png"]);
    expect(state?.index).toBe(0);
  });

  it("returns null for an unrendered Mermaid container, an outsider, and a plain image", () => {
    const root = enhancedRoot();
    const unrendered = document.createElement("div");
    unrendered.className = "mermaid";
    unrendered.dataset.m2hLightboxItem = "true";
    root.append(unrendered);
    expect(collectLightboxState(root, unrendered)).toBeNull();

    const outsider = document.createElement("img");
    outsider.dataset.m2hLightboxItem = "true";
    expect(collectLightboxState(root, outsider)).toBeNull();

    const plain = root.querySelectorAll<HTMLImageElement>("img")[2];
    expect(collectLightboxState(root, plain)).toBeNull();
  });
});
