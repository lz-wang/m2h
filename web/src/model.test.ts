import { describe, expect, it } from "vitest";

import type { FileSummary } from "./api";
import {
  ancestorDirectories,
  buildTree,
  chooseDocument,
  readRoute,
  routeURL,
} from "./model";

const files: FileSummary[] = [
  { path: "z.md", name: "z.md", title: "Z" },
  { path: "guide/part10.md", name: "part10.md", title: "Part 10" },
  { path: "guide/part2.md", name: "part2.md", title: "Part 2" },
];

describe("route model", () => {
  it("reads, normalizes, and writes document routes", () => {
    expect(
      readRoute(
        "/doc/guide/space%20name.md",
        "?mode=dark&width=wide",
        "#install",
      ),
    ).toEqual({
      path: "guide/space name.md",
      mode: "dark",
      width: "wide",
      hash: "#install",
    });
    expect(readRoute("/doc/%", "?mode=invalid")).toEqual({
      path: null,
      mode: "auto",
      width: "standard",
      hash: "",
    });
    expect(readRoute("/", "?mode=light")).toEqual({
      path: null,
      mode: "light",
      width: "standard",
      hash: "",
    });
    expect(routeURL("guide/space name.md", "auto", "standard", "install")).toBe(
      "/doc/guide/space%20name.md#install",
    );
    expect(routeURL("guide/space name.md", "auto", "full", "install")).toBe(
      "/doc/guide/space%20name.md?width=full#install",
    );
    expect(routeURL(null, "light", "standard")).toBe("/?mode=light");
    expect(routeURL(null, "dark", "wide")).toBe("/?mode=dark&width=wide");
  });

  it("chooses the requested, default, first, or empty document", () => {
    expect(chooseDocument(files, "z.md", "guide/part2.md")).toBe("z.md");
    expect(chooseDocument(files, "missing.md", "guide/part2.md")).toBe(
      "guide/part2.md",
    );
    expect(chooseDocument(files, null, "missing.md")).toBe("z.md");
    expect(chooseDocument([], null, "")).toBeNull();
  });
});

describe("tree model", () => {
  it("groups directories first and sorts file names naturally", () => {
    const tree = buildTree(files);
    expect(tree.map((node) => node.name)).toEqual(["guide", "z.md"]);
    const guide = tree[0];
    expect(guide?.type).toBe("directory");
    if (guide?.type === "directory") {
      expect(guide.children.map((node) => node.name)).toEqual([
        "part2.md",
        "part10.md",
      ]);
    }
    expect(ancestorDirectories("one/two/file.md")).toEqual(["one", "one/two"]);
    expect(ancestorDirectories(null)).toEqual([]);
  });
});
