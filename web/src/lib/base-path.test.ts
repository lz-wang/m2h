import { afterEach, describe, expect, it, vi } from "vitest";

afterEach(() => {
  document.head.innerHTML = "";
  vi.unstubAllGlobals();
  vi.resetModules();
});

describe.each(["", "/docs", "/apps/docs"])("deployment prefix %s", (base) => {
  it("generates and parses routes, preserving encoded keys, preferences and anchors", async () => {
    vi.resetModules();
    document.head.innerHTML = `<meta name="m2h-base-path" content="${base}">`;
    const { appPath, stripBasePath } = await import("./base-path");
    const { routeURL, readRoute, documentURL, markdownURL } = await import(
      "../model"
    );
    expect(appPath("/api/files")).toBe(`${base}/api/files`);
    expect(stripBasePath(`${base}/doc/a.md`)).toBe("/doc/a.md");
    expect(stripBasePath(base || "/")).toBe("/");
    if (base !== "") {
      expect(stripBasePath(`${base}-other/doc/a.md`)).toBeNull();
      expect(readRoute("/doc/a.md", "").path).toBeNull();
    }
    const options = { mode: "dark", width: "wide", toc: false } as const;
    const path = "r0/文档/a #%.md";
    const url = routeURL(path, options, "intro");
    expect(url).toBe(
      `${base}/doc/r0/%E6%96%87%E6%A1%A3/a%20%23%25.md?mode=dark&width=wide&toc=false#intro`,
    );
    const parsed = new URL(url, "https://example.com");
    expect(readRoute(parsed.pathname, parsed.search, parsed.hash)).toEqual({
      path,
      ...options,
      hash: "#intro",
    });
    expect(routeURL(null, options)).toBe(
      `${base}/?mode=dark&width=wide&toc=false`,
    );
    expect(documentURL("r1/guide.md", "part")).toBe(
      `${base}/doc/r1/guide.md#part`,
    );
    expect(markdownURL("r1/a b.md")).toBe(`${base}/raw/r1/a%20b.md`);
    expect(readRoute(`${base}/doc/%`, "").path).toBeNull();
  });

  it("prefixes all API requests and loading/failure placeholders", async () => {
    vi.resetModules();
    document.head.innerHTML = `<meta name="m2h-base-path" content="${base}">`;
    const fetch = vi
      .fn()
      .mockResolvedValue(new Response("{}", { status: 200 }));
    vi.stubGlobal("fetch", fetch);
    const { browserAPI } = await import("../api");
    await Promise.allSettled([
      browserAPI.listFiles(),
      browserAPI.getDocument("a b.md"),
      browserAPI.getFileMetadata("a b.md"),
      browserAPI.search("hello world"),
      browserAPI.getMarkdown("a b.md"),
    ]);
    expect(fetch.mock.calls.map(([url]) => url)).toEqual([
      `${base}/api/files`,
      `${base}/api/document?path=a+b.md`,
      `${base}/api/file-metadata?path=a+b.md`,
      `${base}/api/search?q=hello+world`,
      `${base}/raw/a%20b.md`,
    ]);
    const { IMAGE_LOADING_SRC, prepareLazyImages } = await import(
      "./lazy-images"
    );
    expect(IMAGE_LOADING_SRC).toBe(`${base}/ui/image-loading.svg`);
    expect(
      prepareLazyImages('<img src="logo.png">')
        .querySelector("img")
        ?.getAttribute("src"),
    ).toBe(IMAGE_LOADING_SRC);
  });
});

it("defaults to root when the shell supplies no prefix", async () => {
  vi.resetModules();
  document.head.innerHTML = "";
  const { appPath, stripBasePath } = await import("./base-path");
  expect(appPath("/ui/favicon.svg")).toBe("/ui/favicon.svg");
  expect(stripBasePath("/docs-other")).toBe("/docs-other");
});
