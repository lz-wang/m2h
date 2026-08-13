import { afterEach, beforeEach, describe, expect, it } from "vitest";

import { readScrollPosition, saveScrollPosition } from "./scroll-position";

describe("scroll position persistence", () => {
  beforeEach(() => {
    window.sessionStorage.clear();
  });

  afterEach(() => {
    window.sessionStorage.clear();
  });

  it("round-trips a per-document scroll offset through sessionStorage", () => {
    saveScrollPosition("guide/setup.md", 4287);
    expect(readScrollPosition("guide/setup.md")).toBe(4287);

    // Each document keeps its own offset.
    saveScrollPosition("README.md", 120);
    expect(readScrollPosition("guide/setup.md")).toBe(4287);
    expect(readScrollPosition("README.md")).toBe(120);
  });

  it("overwrites the previous offset for the same document", () => {
    saveScrollPosition("README.md", 120);
    saveScrollPosition("README.md", 800);
    expect(readScrollPosition("README.md")).toBe(800);
  });

  it("treats a top position (0) and a missing entry the same: nothing to restore", () => {
    saveScrollPosition("README.md", 0);
    expect(readScrollPosition("README.md")).toBeNull();
    expect(readScrollPosition("never-saved.md")).toBeNull();
  });

  it("ignores corrupted entries", () => {
    window.sessionStorage.setItem("m2h.scroll.README.md", "not-a-number");
    expect(readScrollPosition("README.md")).toBeNull();
  });
});
