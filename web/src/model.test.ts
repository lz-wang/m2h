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

// autoOpenDocument fixtures: a one-line file summary and a one-root workspace.
function file(path: string): FileSummary {
  const name = path.split("/").pop() ?? path;
  return { path, name, title: name };
}

function directoryRoot(rootFiles: FileSummary[]): RootSummary {
  return { id: "r0", name: "docs", files: rootFiles };
}

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

  it("auto-opens the single-file preview's one document", () => {
    // A single-file preview has no sidebar to pick from: its only document
    // opens by itself, wherever it lives.
    expect(autoOpenDocument([directoryRoot(files)], "single")).toBe("z.md");
    expect(
      autoOpenDocument([directoryRoot([files[1] ?? files[0]])], "single"),
    ).toBe("guide/part10.md");
    expect(autoOpenDocument([], "single")).toBeNull();
  });

  it("picks the first root's README over index and any other document", () => {
    expect(
      autoOpenDocument(
        [directoryRoot([file("a.md"), file("index.md"), file("README.md")])],
        "directory",
      ),
    ).toBe("README.md");
    expect(
      autoOpenDocument(
        [directoryRoot([file("a.md"), file("index.md")])],
        "directory",
      ),
    ).toBe("index.md");
  });

  it("matches README and index case-insensitively and by name only", () => {
    // Title casing and uppercase extensions do not hide an entry document…
    expect(
      autoOpenDocument(
        [directoryRoot([file("Readme.MD"), file("a.md")])],
        "directory",
      ),
    ).toBe("Readme.MD");
    expect(
      autoOpenDocument(
        [directoryRoot([file("INDEX.md"), file("a.md")])],
        "directory",
      ),
    ).toBe("INDEX.md");
    // … but a same-stem title never promotes a non-README name.
    expect(
      autoOpenDocument(
        [directoryRoot([{ ...file("a.md"), title: "README" }])],
        "directory",
      ),
    ).toBe("a.md");
  });

  it("falls back to the first root-level file in natural name order", () => {
    // b.md before a.md as served is irrelevant: the pick sorts by name, and
    // numeric order keeps numbered notes in reading sequence.
    expect(
      autoOpenDocument(
        [directoryRoot([file("b.md"), file("a.md")])],
        "directory",
      ),
    ).toBe("a.md");
    expect(
      autoOpenDocument(
        [
          directoryRoot([
            file("10-extra.md"),
            file("02-guide.md"),
            file("01-intro.md"),
          ]),
        ],
        "directory",
      ),
    ).toBe("01-intro.md");
  });

  it("never picks a subdirectory README or index", () => {
    // docs/README.md is not the workspace's entry document…
    expect(
      autoOpenDocument(
        [directoryRoot([file("docs/README.md"), file("a.md")])],
        "directory",
      ),
    ).toBe("a.md");
    // … and when only nested documents exist, nothing opens.
    expect(
      autoOpenDocument([directoryRoot([file("docs/index.md")])], "directory"),
    ).toBeNull();
  });

  it("opens nothing for an empty root", () => {
    expect(autoOpenDocument([directoryRoot([])], "directory")).toBeNull();
    expect(autoOpenDocument([], "directory")).toBeNull();
    expect(autoOpenDocument([], "workspace")).toBeNull();
  });

  it("consults only the first root of a multi-root workspace and returns virtual paths", () => {
    // The first root's README wins over the second root's index.
    expect(
      autoOpenDocument(
        [
          { id: "root-a", name: "root-a", files: [file("README.md")] },
          { id: "root-b", name: "root-b", files: [file("index.md")] },
        ],
        "workspace",
      ),
    ).toBe("root-a/README.md");
    // A first root without root-level documents never falls through to the
    // second root's README.
    expect(
      autoOpenDocument(
        [
          { id: "root-a", name: "root-a", files: [file("nested/guide.md")] },
          { id: "root-b", name: "root-b", files: [file("README.md")] },
        ],
        "workspace",
      ),
    ).toBeNull();
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
