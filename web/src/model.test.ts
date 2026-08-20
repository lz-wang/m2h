import { describe, expect, it } from "vitest";

import type { FileSummary, RootSummary } from "./api";
import {
  ancestorDirectories,
  buildTree,
  chooseDocument,
  decodeHeadingHash,
  encodeHeadingHash,
  readRoute,
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
    const roots: RootSummary[] = [{ id: "r0", name: "docs", files: [readme] }];
    expect(rootFiles(roots)).toEqual([readme]);
    expect(rootFiles([])).toEqual([]);
  });

  it("prefixes every file with its root id in a multi-root workspace", () => {
    const roots: RootSummary[] = [
      { id: "r0", name: "alpha", files: [readme] },
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

  it("composes the default document key per workspace shape", () => {
    expect(rootDocumentKey(null, false)).toBe("");
    expect(rootDocumentKey({ root: "r0", path: "README.md" }, false)).toBe(
      "README.md",
    );
    expect(rootDocumentKey({ root: "r0", path: "README.md" }, true)).toBe(
      "r0/README.md",
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
