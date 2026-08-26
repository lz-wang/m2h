import { beforeEach, describe, expect, it, vi } from "vitest";

import { browserAPI } from "./api";

const fetchMock = vi.fn<typeof fetch>();

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockReset();
});

describe("browser API", () => {
  it("validates file and document responses and encodes paths", async () => {
    fetchMock
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            kind: "directory",
            version: "0.9.1",
            roots: [
              {
                id: "r0",
                name: "docs",
                files: [
                  { path: "README.md", name: "README.md", title: "Readme" },
                ],
              },
            ],
            defaultDocument: { root: "r0", path: "README.md" },
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            path: "space name.md",
            title: "Space",
            html: "<p>Body</p>",
            frontmatter: {
              entries: [{ key: "date", value: "2026-07-11" }],
              date: "2026-07-11",
              tags: ["Go"],
            },
            toc: [
              { level: 2, id: "install", text: "Install" },
              { level: 3, id: "homebrew", text: "Homebrew" },
            ],
          }),
          {
            status: 200,
            headers: { "Content-Type": "application/json" },
          },
        ),
      );

    await expect(browserAPI.listFiles()).resolves.toEqual({
      kind: "directory",
      version: "0.9.1",
      roots: [
        {
          id: "r0",
          name: "docs",
          files: [{ path: "README.md", name: "README.md", title: "Readme" }],
        },
      ],
      defaultDocument: { root: "r0", path: "README.md" },
    });
    await expect(browserAPI.getDocument("space name.md")).resolves.toEqual({
      path: "space name.md",
      title: "Space",
      html: "<p>Body</p>",
      frontmatter: {
        entries: [{ key: "date", value: "2026-07-11" }],
        date: "2026-07-11",
        tags: ["Go"],
      },
      toc: [
        { level: 2, id: "install", text: "Install" },
        { level: 3, id: "homebrew", text: "Homebrew" },
      ],
    });
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      "/api/document?path=space+name.md",
    );
  });

  it("normalizes the preview kind", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          kind: "single",
          version: "dev-20260812-abcdef0",
          roots: [
            {
              id: "r0",
              name: "README.md",
              files: [
                { path: "README.md", name: "README.md", title: "Readme" },
              ],
            },
          ],
          defaultDocument: { root: "r0", path: "README.md" },
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.listFiles()).resolves.toMatchObject({
      kind: "single",
    });

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          kind: "workspace",
          version: "dev-20260812-abcdef0",
          roots: [
            {
              id: "r0",
              name: "a",
              files: [
                { path: "README.md", name: "README.md", title: "Readme" },
              ],
            },
            {
              id: "r1",
              name: "b",
              files: [],
            },
          ],
          defaultDocument: { root: "r0", path: "README.md" },
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.listFiles()).resolves.toMatchObject({
      kind: "workspace",
      roots: [{ id: "r0" }, { id: "r1" }],
    });

    // A missing or unrecognized kind falls back to directory so the WebUI keeps
    // the richer navigation UI when the server contract is uncertain.
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          version: "dev-20260812-abcdef0",
          roots: [
            {
              id: "r0",
              name: "docs",
              files: [
                { path: "README.md", name: "README.md", title: "Readme" },
              ],
            },
          ],
          defaultDocument: null,
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.listFiles()).resolves.toMatchObject({
      kind: "directory",
      defaultDocument: null,
    });
  });

  it("surfaces JSON HTTP errors", async () => {
    fetchMock.mockResolvedValue(
      new Response(JSON.stringify({ error: "missing" }), {
        status: 404,
        statusText: "Not Found",
        headers: { "Content-Type": "application/json" },
      }),
    );
    await expect(browserAPI.getDocument("missing.md")).rejects.toMatchObject({
      name: "APIError",
      status: 404,
      message: "missing",
    });
  });

  it("rejects malformed successful responses", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ roots: "invalid", version: "1" }), {
        status: 200,
      }),
    );
    await expect(browserAPI.listFiles()).rejects.toThrow(
      "文件列表响应格式无效",
    );

    // Root summaries and the default document reference are validated to the
    // same strictness as the files themselves.
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          version: "1",
          roots: [
            {
              id: "r0",
              name: "docs",
              files: [{ path: "README.md" }],
            },
          ],
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.listFiles()).rejects.toThrow(
      "文件条目响应格式无效",
    );

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          version: "1",
          roots: [{ id: "r0", name: "docs", files: [] }],
          defaultDocument: { root: "r0" },
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.listFiles()).rejects.toThrow(
      "文件列表响应格式无效",
    );

    // id, name and files are each required strings/array: a root missing any
    // of them never reaches the UI.
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          version: "1",
          roots: [{ id: "r0", files: [] }],
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.listFiles()).rejects.toThrow(
      "文件列表响应格式无效",
    );

    fetchMock.mockResolvedValueOnce(
      new Response(JSON.stringify({ path: "a.md", title: 1, html: "" }), {
        status: 200,
      }),
    );
    await expect(browserAPI.getDocument("a.md")).rejects.toThrow(
      "文档响应格式无效",
    );
  });

  it("fetches raw Markdown through the encoded /raw/ address", async () => {
    const source = "---\ntitle: Raw\n---\n# Raw\n\nbody\n";
    fetchMock.mockResolvedValueOnce(
      new Response(source, {
        status: 200,
        headers: { "Content-Type": "text/markdown; charset=utf-8" },
      }),
    );
    await expect(browserAPI.getMarkdown("docs/a b.md")).resolves.toBe(source);
    expect(fetchMock.mock.calls[0]?.[0]).toBe("/raw/docs/a%20b.md");
    expect(fetchMock.mock.calls[0]?.[1]).toMatchObject({
      headers: { Accept: "text/markdown" },
    });

    // Multi-root keys keep their root prefix, Unicode segments stay encoded.
    fetchMock.mockResolvedValueOnce(new Response("# Beta", { status: 200 }));
    await expect(browserAPI.getMarkdown("r1/计划.md")).resolves.toBe("# Beta");
    expect(fetchMock.mock.calls[1]?.[0]).toBe(
      `/raw/r1/${encodeURIComponent("计划.md")}`,
    );
  });

  it("surfaces raw Markdown HTTP errors without a JSON body", async () => {
    // /raw/ answers errors as plain text (http.Error on the server), so only
    // the status is available to the caller.
    fetchMock.mockResolvedValueOnce(
      new Response("document not found\n", {
        status: 404,
        statusText: "Not Found",
        headers: { "Content-Type": "text/plain" },
      }),
    );
    await expect(browserAPI.getMarkdown("missing.md")).rejects.toMatchObject({
      name: "APIError",
      status: 404,
    });
  });

  it("accepts null, missing, and malformed frontmatter", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          path: "null.md",
          title: "Null",
          html: "<p>Body</p>",
          frontmatter: null,
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.getDocument("null.md")).resolves.toEqual({
      path: "null.md",
      title: "Null",
      html: "<p>Body</p>",
      frontmatter: null,
      toc: [],
    });

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({ path: "missing.md", title: "Missing", html: "" }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.getDocument("missing.md")).resolves.toEqual({
      path: "missing.md",
      title: "Missing",
      html: "",
      frontmatter: null,
      toc: [],
    });

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          path: "bad.md",
          title: "Bad",
          html: "",
          frontmatter: { entries: "nope" },
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.getDocument("bad.md")).rejects.toThrow(
      "文档响应格式无效",
    );
  });

  it("validates the table of contents shape", async () => {
    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          path: "toc.md",
          title: "Toc",
          html: "<p>Body</p>",
          toc: [{ level: "two", id: "install", text: "Install" }],
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.getDocument("toc.md")).rejects.toThrow(
      "文档响应格式无效",
    );

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          path: "notarray.md",
          title: "NotArray",
          html: "<p>Body</p>",
          toc: { level: 2, id: "install", text: "Install" },
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.getDocument("notarray.md")).rejects.toThrow(
      "文档响应格式无效",
    );

    fetchMock.mockResolvedValueOnce(
      new Response(
        JSON.stringify({
          path: "missing-fields.md",
          title: "Missing",
          html: "<p>Body</p>",
          toc: [{ level: 2, id: "install" }],
        }),
        { status: 200 },
      ),
    );
    await expect(browserAPI.getDocument("missing-fields.md")).rejects.toThrow(
      "文档响应格式无效",
    );
  });
});
