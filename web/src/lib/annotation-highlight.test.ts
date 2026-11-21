import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ANNOTATION_HIGHLIGHT_NAME,
  AnnotationHighlightLayer,
  HighlightLayer,
  supportsHighlights,
} from "./annotation-highlight";
import { TextIndex } from "./annotation-selector";

function mountBody(html: string): HTMLElement {
  document.body.replaceChildren();
  const root = document.createElement("article");
  root.innerHTML = html;
  document.body.append(root);
  return root;
}

class FakeHighlight {
  ranges: Range[];
  constructor(...ranges: Range[]) {
    this.ranges = ranges;
  }
}

function installFakeRegistry() {
  const store = new Map<string, FakeHighlight>();
  const registry = {
    set: vi.fn((name: string, highlight: FakeHighlight) => {
      store.set(name, highlight);
      return registry;
    }),
    delete: vi.fn((name: string) => store.delete(name)),
  };
  const globalCSS = globalThis.CSS as unknown as { highlights?: unknown };
  const globalHighlight = (globalThis as { Highlight?: unknown }).Highlight;
  globalCSS.highlights = registry;
  (globalThis as { Highlight?: unknown }).Highlight = FakeHighlight;
  return {
    registry,
    store,
    restore() {
      globalCSS.highlights = undefined;
      (globalThis as { Highlight?: unknown }).Highlight = globalHighlight;
    },
  };
}

afterEach(() => {
  document.body.replaceChildren();
});

describe("AnnotationHighlightLayer", () => {
  it("paints only the newest color across overlaps and moves emphasized spans to their own group", () => {
    const fake = installFakeRegistry();
    try {
      const layer = new AnnotationHighlightLayer();
      layer.render(new TextIndex(mountBody("<p>abcdefghij</p>")), [
        { id: "one", color: "blue", position: { start: 0, end: 8 } },
        { id: "two", color: "green", position: { start: 2, end: 6 } },
        { id: "three", color: "amber", position: { start: 4, end: 5 } },
      ]);
      const textOf = (name: string) =>
        fake.store.get(name)?.ranges.map((range) => range.toString());
      expect(textOf("m2h-annotations")).toEqual(["ab", "gh"]);
      expect(textOf("m2h-annotations-green")).toEqual(["cd", "f"]);
      expect(textOf("m2h-annotations-amber")).toEqual(["e"]);

      layer.emphasize("two", "three");
      expect(textOf("m2h-annotations-green")).toBeUndefined();
      expect(textOf("m2h-annotations-green-active")).toEqual(["cd", "f"]);
      expect(textOf("m2h-annotations-amber-active")).toEqual(["e"]);
      layer.release();
      expect(fake.store.size).toBe(0);
    } finally {
      fake.restore();
    }
  });
});

describe("HighlightLayer", () => {
  it("degrades silently without the Highlight API", () => {
    expect(supportsHighlights()).toBe(false);
    const layer = new HighlightLayer();
    expect(() => {
      layer.attach(new TextIndex(mountBody("<p>x</p>")));
      layer.render([{ start: 0, end: 1 }]);
      layer.release();
    }).not.toThrow();
  });

  it("registers one group of resolved ranges", () => {
    const fake = installFakeRegistry();
    try {
      const root = mountBody("<p>标注目标文本</p>");
      const index = new TextIndex(root);
      const layer = new HighlightLayer();
      layer.attach(index);
      layer.render([
        { start: 0, end: 2 },
        { start: 2, end: 4 },
      ]);
      expect(layer.isRegistered).toBe(true);
      const highlight = fake.store.get(ANNOTATION_HIGHLIGHT_NAME);
      expect(highlight?.ranges.length).toBe(2);
      expect(highlight?.ranges[0]?.toString()).toBe("标注");

      layer.render([]);
      expect(fake.store.has(ANNOTATION_HIGHLIGHT_NAME)).toBe(false);
      expect(layer.isRegistered).toBe(false);
    } finally {
      fake.restore();
    }
  });

  it("release drops the group even without an attached index", () => {
    const fake = installFakeRegistry();
    try {
      const layer = new HighlightLayer();
      layer.attach(new TextIndex(mountBody("<p>abc</p>")));
      layer.render([{ start: 0, end: 3 }]);
      layer.attach(null);
      layer.release();
      expect(fake.store.has(ANNOTATION_HIGHLIGHT_NAME)).toBe(false);
    } finally {
      fake.restore();
    }
  });

  it("renders nothing while detached", () => {
    const fake = installFakeRegistry();
    try {
      const layer = new HighlightLayer();
      layer.render([{ start: 0, end: 1 }]);
      expect(fake.store.has(ANNOTATION_HIGHLIGHT_NAME)).toBe(false);
    } finally {
      fake.restore();
    }
  });
});
