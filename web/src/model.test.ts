import { describe, expect, it } from "vitest";

import type { FileSummary, RootSummary } from "./api";
import {
  absoluteURL,
  ancestorDirectories,
  buildTree,
  chooseDocument,
  decodeHeadingHash,
  documentURL,
  encodeHeadingHash,
  encodeVirtualPath,
  localDocumentPath,
  localPath,
  markdownURL,
  readRoute,
  resolveDocumentLocation,
  rootDocumentKey,
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

  it("chooses the requested, default, first, or empty document", () => {
    expect(chooseDocument(files, "z.md", "guide/part2.md")).toBe("z.md");
    expect(chooseDocument(files, "missing.md", "guide/part2.md")).toBe(
      "guide/part2.md",
    );
    expect(chooseDocument(files, null, "missing.md")).toBe("z.md");
    expect(chooseDocument([], null, "")).toBeNull();
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
        kind: "directory",
        absolutePath: "/tmp/docs",
        pathSeparator: "/",
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
        kind: "directory",
        absolutePath: "/tmp/alpha",
        pathSeparator: "/",
        files: [readme],
      },
      {
        id: "r1",
        name: "beta",
        kind: "directory",
        absolutePath: "D:\\work\\beta",
        pathSeparator: "\\",
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

  it("composes the default document key per workspace shape", () => {
    expect(rootDocumentKey(null, false)).toBe("");
    expect(rootDocumentKey({ root: "r0", path: "README.md" }, false)).toBe(
      "README.md",
    );
    expect(rootDocumentKey({ root: "r0", path: "README.md" }, true)).toBe(
      "r0/README.md",
    );
  });

  it("resolves single- and multi-root document locations", () => {
    const singleRoots: RootSummary[] = [
      {
        id: "r0",
        name: "docs",
        kind: "directory",
        absolutePath: "/tmp/docs",
        pathSeparator: "/",
        files: [
          readme,
          { path: "guide/part.md", name: "part.md", title: "Part" },
        ],
      },
    ];
    const single = resolveDocumentLocation(singleRoots, "guide/part.md");
    expect(single?.root.id).toBe("r0");
    expect(single?.file.path).toBe("guide/part.md");
    expect(single?.relativePath).toBe("guide/part.md");
    // A single root treats the whole key as relative — no id prefix involved.
    expect(resolveDocumentLocation(singleRoots, "r0/guide/part.md")).toBeNull();

    const multiRoots: RootSummary[] = [
      {
        id: "r0",
        name: "alpha",
        kind: "directory",
        absolutePath: "/tmp/alpha",
        pathSeparator: "/",
        files: [readme],
      },
      {
        id: "r1",
        name: "beta",
        kind: "directory",
        absolutePath: "/tmp/beta",
        pathSeparator: "/",
        files: [{ path: "README.md", name: "README.md", title: "Beta Readme" }],
      },
    ];
    // Same-named documents in two roots resolve to their own root.
    const alpha = resolveDocumentLocation(multiRoots, "r0/README.md");
    expect(alpha?.root.id).toBe("r0");
    expect(alpha?.file.title).toBe("Readme");
    expect(alpha?.relativePath).toBe("README.md");
    const beta = resolveDocumentLocation(multiRoots, "r1/README.md");
    expect(beta?.root.id).toBe("r1");
    expect(beta?.file.title).toBe("Beta Readme");

    // Unknown roots, bare keys and missing documents resolve nowhere.
    expect(resolveDocumentLocation(multiRoots, "r2/README.md")).toBeNull();
    expect(resolveDocumentLocation(multiRoots, "README.md")).toBeNull();
    expect(resolveDocumentLocation(multiRoots, "r0")).toBeNull();
    expect(resolveDocumentLocation(multiRoots, "r0/missing.md")).toBeNull();
    expect(resolveDocumentLocation(multiRoots, null)).toBeNull();
    expect(resolveDocumentLocation([], null)).toBeNull();
  });

  it("builds local document paths per root kind", () => {
    const directoryRoot: RootSummary = {
      id: "r0",
      name: "docs",
      kind: "directory",
      absolutePath: "/tmp/docs",
      pathSeparator: "/",
      files: [readme],
    };
    expect(localDocumentPath(directoryRoot, "guide/part.md")).toBe(
      "/tmp/docs/guide/part.md",
    );
    const windowsRoot: RootSummary = {
      ...directoryRoot,
      absolutePath: "D:\\work\\docs",
      pathSeparator: "\\",
    };
    expect(localDocumentPath(windowsRoot, "guide/part.md")).toBe(
      "D:\\work\\docs\\guide\\part.md",
    );
    // A file root's absolutePath already names the file: appending the only
    // document's path again would produce /docs/solo.md/solo.md.
    const fileRoot: RootSummary = {
      id: "r0",
      name: "solo.md",
      kind: "file",
      absolutePath: "/tmp/docs/solo.md",
      pathSeparator: "/",
      files: [{ path: "solo.md", name: "solo.md", title: "Solo" }],
    };
    expect(localDocumentPath(fileRoot, "solo.md")).toBe("/tmp/docs/solo.md");
    // The shared joiner honors the server-reported separator and a root that
    // already ends with one.
    expect(localPath("/tmp/docs", "", "/")).toBe("/tmp/docs");
    expect(localPath("/tmp/docs/", "a.md", "/")).toBe("/tmp/docs/a.md");
    expect(localPath("D:\\docs", "a/b.md", "\\")).toBe("D:\\docs\\a\\b.md");
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
});
