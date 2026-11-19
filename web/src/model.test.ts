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
  type TreeNode,
} from "./model";

const files: FileSummary[] = [
  { path: "z.md", name: "z.md" },
  { path: "guide/part10.md", name: "part10.md" },
  { path: "guide/part2.md", name: "part2.md" },
];

// autoOpenDocument fixtures: a one-line file summary and a one-root workspace.
function file(path: string): FileSummary {
  const name = path.split("/").pop() ?? path;
  return { path, name };
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

  it("reserves directory and workspace roots for the home route", () => {
    const root = directoryRoot([
      file("README.md"),
      file("index.md"),
      file("a.md"),
    ]);
    expect(autoOpenDocument([root], "directory")).toBeNull();
    expect(
      autoOpenDocument([root, { ...root, id: "r1" }], "workspace"),
    ).toBeNull();
    expect(autoOpenDocument([directoryRoot([])], "single")).toBeNull();
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
          { path: "README.md", name: "README.md" },
          {
            path: "guide/part.md",
            name: "part.md",
          },
        ],
      },
    ];
    expect(rootFiles(roots)).toEqual([
      { ...readme, path: "r0/README.md" },
      { path: "r1/README.md", name: "README.md" },
      { path: "r1/guide/part.md", name: "part.md" },
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

  it("merges nested paths through shared directory nodes", () => {
    const tree = buildTree([
      { path: "a/b/one.md", name: "one.md" },
      { path: "a/b/two.md", name: "two.md" },
      { path: "a/c.md", name: "c.md" },
      { path: "top.md", name: "top.md" },
    ]);
    expect(tree.map((node) => node.name)).toEqual(["a", "top.md"]);
    const directory = tree[0];
    expect(directory?.type).toBe("directory");
    if (directory?.type !== "directory") {
      throw new Error("expected a directory node");
    }
    expect(directory.children.map((node) => node.name)).toEqual(["b", "c.md"]);
    const nested = directory.children[0];
    if (nested?.type !== "directory") {
      throw new Error("expected a directory node");
    }
    expect(nested.children.map((node) => node.path)).toEqual([
      "a/b/one.md",
      "a/b/two.md",
    ]);
  });

  it("keeps a same-named file and directory as separate siblings", () => {
    const tree = buildTree([
      { path: "x", name: "x" },
      { path: "x/inner.md", name: "inner.md" },
    ]);
    // Directories sort before files, so the file "x" and the directory "x"
    // both exist as distinct siblings.
    expect(tree.map((node) => node.type)).toEqual(["directory", "file"]);
    const directory = tree[0];
    if (directory?.type !== "directory") {
      throw new Error("expected a directory node");
    }
    expect(directory.children.map((node) => node.path)).toEqual(["x/inner.md"]);
  });

  it("keeps structure and node counts on a 10000-file synthetic listing", () => {
    // Scale smoke test, deliberately without timing assertions: CI machines
    // jitter too much for duration to be meaningful. It pins the contract
    // that matters — construction stays correct at 10K entries — and the
    // algorithmic cost itself is covered by the shape assertions.
    const listing: FileSummary[] = [];
    for (let group = 0; group < 100; group += 1) {
      const groupDirectory = `group-${String(group).padStart(3, "0")}`;
      for (let doc = 0; doc < 100; doc += 1) {
        const name = `doc-${String(doc).padStart(4, "0")}.md`;
        listing.push({
          path: `${groupDirectory}/sub/${name}`,
          name,
        });
      }
    }
    const tree = buildTree(listing);
    expect(tree).toHaveLength(100);
    let files = 0;
    let directories = 0;
    const walk = (nodes: TreeNode[]) => {
      for (const node of nodes) {
        if (node.type === "directory") {
          directories += 1;
          walk(node.children);
        } else {
          files += 1;
        }
      }
    };
    walk(tree);
    expect(files).toBe(10_000);
    expect(directories).toBe(200);
    // Natural numeric order holds deep in the tree.
    const first = tree[0];
    if (first?.type !== "directory") {
      throw new Error("expected a directory node");
    }
    const sub = first.children[0];
    if (sub?.type !== "directory") {
      throw new Error("expected a directory node");
    }
    expect(sub.children[0]?.name).toBe("doc-0000.md");
    expect(sub.children[9_999 % 100]?.name).toBe("doc-0099.md");
  });

  it("expands first-level directories of an unselected single-root tree", () => {
    const tree = buildTree([
      { path: "docs/a/b.md", name: "b.md" },
      { path: "docs/deep/c/d.md", name: "d.md" },
      { path: "guide.md", name: "guide.md" },
    ]);
    // Top-level directories open, the deeper "docs/deep" level stays closed.
    expect(initialExpandedPaths(tree, null, false, "")).toEqual(
      new Set(["docs"]),
    );
  });

  it("expands only the selection's own chain, root row included", () => {
    const tree = buildTree([
      { path: "a/b/c.md", name: "c.md" },
      { path: "x/y.md", name: "y.md" },
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
    const tree = buildTree([{ path: "README.md", name: "README.md" }]);
    expect(initialExpandedPaths(tree, null, true, "r0")).toEqual(new Set());
  });
});

describe("workspace-scale model", () => {
  // 10K synthetic documents across two roots, mirroring the shape the
  // /api/files listing serves at scale. Like the buildTree smoke test these
  // assertions pin correctness, not speed — benchmarks own the numbers.
  function listing(prefix: string, count: number): FileSummary[] {
    const files: FileSummary[] = [];
    for (let index = 0; index < count; index += 1) {
      const name = `doc-${String(index).padStart(5, "0")}.md`;
      files.push({
        path: `${prefix}/group-${index % 40}/${name}`,
        name,
      });
    }
    return files;
  }

  const workspace: RootSummary[] = [
    { id: "r0", name: "alpha", files: listing("group", 5_000) },
    { id: "r1", name: "beta", files: listing("notes", 5_000) },
  ];

  it("flattens and prefixes a 10000-file multi-root workspace", () => {
    const files = rootFiles(workspace);
    expect(files).toHaveLength(10_000);
    // Every virtual key is unique and keeps the "<rootId>/<path>" shape the
    // document API, scroll storage and sidebar selection all share.
    const keys = new Set(files.map((file) => file.path));
    expect(keys.size).toBe(10_000);
    expect(keys.has("r0/group/group-7/doc-00007.md")).toBe(true);
    expect(keys.has("r1/notes/group-7/doc-00007.md")).toBe(true);
  });
});
