import { describe, expect, it } from "vitest";

import type { FileSummary, RootSummary } from "./api";
import {
  absoluteURL,
  ancestorDirectories,
  autoOpenDocument,
  buildTree,
  decodeHeadingHash,
  documentURL,
  encodeHeadingHash,
  encodeVirtualPath,
  initialExpandedPaths,
  markdownURL,
  readRoute,
  rootFiles,
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
      toc: true,
      hash: "#install",
    });
    expect(readRoute("/doc/%", "?mode=invalid")).toEqual({
      path: null,
      mode: "auto",
      width: "standard",
      toc: true,
      hash: "",
    });
    expect(readRoute("/", "?mode=light")).toEqual({
      path: null,
      mode: "light",
      width: "standard",
      toc: true,
      hash: "",
    });
    expect(readRoute("/doc/guide.md", "?toc=false")).toEqual({
      path: "guide.md",
      mode: "auto",
      width: "standard",
      toc: false,
      hash: "",
    });
    // Unknown toc values fall back to the default (enabled), only the literal
    // "false" disables it.
    expect(readRoute("/doc/guide.md", "?toc=true").toc).toBe(true);
    expect(readRoute("/doc/guide.md", "?toc=0").toc).toBe(true);
    expect(
      routeURL(
        "guide/space name.md",
        { mode: "auto", width: "standard", toc: true },
        "install",
      ),
    ).toBe("/doc/guide/space%20name.md#install");
    expect(
      routeURL(
        "guide/space name.md",
        { mode: "auto", width: "full", toc: true },
        "install",
      ),
    ).toBe("/doc/guide/space%20name.md?width=full#install");
    expect(
      routeURL(null, { mode: "light", width: "standard", toc: true }),
    ).toBe("/?mode=light");
    expect(routeURL(null, { mode: "dark", width: "wide", toc: true })).toBe(
      "/?mode=dark&width=wide",
    );
    expect(
      routeURL("guide/space name.md", {
        mode: "auto",
        width: "standard",
        toc: false,
      }),
    ).toBe("/doc/guide/space%20name.md?toc=false");
    expect(routeURL(null, { mode: "dark", width: "wide", toc: false })).toBe(
      "/?mode=dark&width=wide&toc=false",
    );
  });

  it("auto-opens only the single-file preview's one document", () => {
    // A single-file preview has no sidebar to pick from: its only document
    // opens by itself.
    expect(autoOpenDocument(files, "single")).toBe("z.md");
    expect(autoOpenDocument([files[1] ?? files[0]], "single")).toBe(
      "guide/part10.md",
    );
    // Directory and multi-root workspaces never pick for the reader, and an
    // empty workspace has nothing to open.
    expect(autoOpenDocument(files, "directory")).toBeNull();
    expect(autoOpenDocument(files, "workspace")).toBeNull();
    expect(autoOpenDocument([], "single")).toBeNull();
  });
});

describe("heading hash codec", () => {
  it("percent-encodes ids and round-trips through the address bar", () => {
    expect(encodeHeadingHash("install")).toBe("#install");
    expect(encodeHeadingHash("安装")).toBe(`#${encodeURIComponent("安装")}`);
    expect(encodeHeadingHash("foo bar-1")).toBe("#foo%20bar-1");

    expect(decodeHeadingHash("#install")).toBe("install");
    expect(decodeHeadingHash(encodeHeadingHash("安装"))).toBe("安装");
    expect(decodeHeadingHash("foo%20bar-1")).toBe("foo bar-1");
    expect(decodeHeadingHash("")).toBe("");
    expect(decodeHeadingHash("#")).toBe("");
  });

  it("keeps the literal fragment when it is not valid percent encoding", () => {
    expect(decodeHeadingHash("#%")).toBe("%");
  });
});

describe("workspace model", () => {
  const readme: FileSummary = {
    path: "README.md",
    name: "README.md",
    title: "Readme",
  };

  it("keeps single-root file paths unprefixed", () => {
    const roots: RootSummary[] = [
      {
        id: "r0",
        name: "docs",
        files: [readme],
      },
    ];
    expect(rootFiles(roots)).toEqual([readme]);
    expect(rootFiles([])).toEqual([]);
  });

  it("prefixes every file with its root id in a multi-root workspace", () => {
    const roots: RootSummary[] = [
      {
        id: "r0",
        name: "alpha",
        files: [readme],
      },
      {
        id: "r1",
        name: "beta",
        files: [
          { path: "README.md", name: "README.md", title: "Beta Readme" },
          {
            path: "guide/part.md",
            name: "part.md",
            title: "Part",
          },
        ],
      },
    ];
    expect(rootFiles(roots)).toEqual([
      { ...readme, path: "r0/README.md" },
      { path: "r1/README.md", name: "README.md", title: "Beta Readme" },
      { path: "r1/guide/part.md", name: "part.md", title: "Part" },
    ]);
  });
});

describe("share URLs", () => {
  it("encodes virtual paths segment by segment", () => {
    expect(documentURL("docs/a b.md")).toBe("/doc/docs/a%20b.md");
    expect(markdownURL("docs/a b.md")).toBe("/raw/docs/a%20b.md");
    // Unicode, "#", "%" and multi-root prefixes all survive the address bar.
    expect(documentURL("计划/#标题.md")).toBe(
      `/doc/${encodeURIComponent("计划")}/${encodeURIComponent("#标题.md")}`,
    );
    expect(markdownURL("计划/#标题.md")).toBe(
      `/raw/${encodeURIComponent("计划")}/${encodeURIComponent("#标题.md")}`,
    );
    expect(markdownURL("rates/50%.md")).toBe(
      `/raw/${encodeURIComponent("rates")}/${encodeURIComponent("50%.md")}`,
    );
    expect(documentURL("r1/docs/guide.md")).toBe("/doc/r1/docs/guide.md");
    expect(markdownURL("r1/docs/guide.md")).toBe("/raw/r1/docs/guide.md");
    // The heading hash is kept (share the reading position) while UI
    // preferences never enter a share URL.
    expect(documentURL("guide.md", "#install")).toBe("/doc/guide.md#install");
    expect(documentURL("guide.md", "install")).toBe("/doc/guide.md#install");
    expect(documentURL("guide.md", "")).toBe("/doc/guide.md");
    expect(encodeVirtualPath("a/b.md")).toBe("a/b.md");
  });

  it("resolves share URLs against an origin through the URL API", () => {
    expect(absoluteURL("/doc/a%20b.md", "http://127.0.0.1:8793")).toBe(
      "http://127.0.0.1:8793/doc/a%20b.md",
    );
    expect(absoluteURL("/raw/计划.md", "http://192.168.1.4:8793/")).toBe(
      `http://192.168.1.4:8793/raw/${encodeURIComponent("计划.md")}`,
    );
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

  it("expands first-level directories of an unselected single-root tree", () => {
    const tree = buildTree([
      { path: "docs/a/b.md", name: "b.md", title: "B" },
      { path: "docs/deep/c/d.md", name: "d.md", title: "D" },
      { path: "guide.md", name: "guide.md", title: "Guide" },
    ]);
    // Top-level directories open, the deeper "docs/deep" level stays closed.
    expect(initialExpandedPaths(tree, null, false, "")).toEqual(
      new Set(["docs"]),
    );
  });

  it("expands only the selection's own chain, root row included", () => {
    const tree = buildTree([
      { path: "a/b/c.md", name: "c.md", title: "C" },
      { path: "x/y.md", name: "y.md", title: "Y" },
    ]);
    // Single root: just the ancestors, no synthetic root row.
    expect(initialExpandedPaths(tree, "a/b/c.md", false, "")).toEqual(
      new Set(["a", "a/b"]),
    );
    // Multi-root owning tree: the root row joins the ancestors.
    expect(initialExpandedPaths(tree, "a/b/c.md", true, "r1")).toEqual(
      new Set(["r1", "a", "a/b"]),
    );
  });

  it("expands nothing in an unselected multi-root tree", () => {
    const tree = buildTree([
      { path: "README.md", name: "README.md", title: "Readme" },
    ]);
    expect(initialExpandedPaths(tree, null, true, "r0")).toEqual(new Set());
  });
});
