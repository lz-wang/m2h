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
            files: [{ path: "README.md", name: "README.md", title: "Readme" }],
            defaultPath: "README.md",
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
      files: [{ path: "README.md", name: "README.md", title: "Readme" }],
      defaultPath: "README.md",
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
      new Response(JSON.stringify({ files: "invalid", defaultPath: "" }), {
        status: 200,
      }),
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
