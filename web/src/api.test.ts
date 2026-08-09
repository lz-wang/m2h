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
});
