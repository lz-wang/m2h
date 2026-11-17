import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AnnotationItem } from "../api";
import { useAnnotations, type AnnotationIdentity } from "./use-annotations";

const fetchMock = vi.fn<typeof fetch>();

function jsonResponse(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), {
    status,
    headers: { "Content-Type": "application/json" },
  });
}

const identity: AnnotationIdentity = {
  path: "design.md",
  documentRef: { rootId: "root-1", path: "design.md" },
  documentHash: "sha256:abc",
};

function item(overrides: Partial<AnnotationItem> = {}): AnnotationItem {
  return {
    id: "a1",
    authorId: "local:alice",
    authorName: "alice",
    comment: "评论",
    quote: "使用 SQLite",
    selector: {
      quote: { exact: "使用 SQLite", prefix: "", suffix: "" },
      position: { start: 0, end: 9 },
      version: 1,
    },
    documentHash: "sha256:abc",
    mine: true,
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  };
}

beforeEach(() => {
  vi.stubGlobal("fetch", fetchMock);
  fetchMock.mockReset();
});

describe("useAnnotations", () => {
  it("loads the list for the current document and resets on switch", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ annotations: [item()] }));
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.items).toHaveLength(1));
    expect(result.current.loading).toBe(false);

    rerender({ identity: { ...identity, path: "other.md" } });
    // The old document's annotations never bleed into the new one.
    expect(result.current.items).toHaveLength(0);
  });

  it("stays empty while disabled", async () => {
    const { result } = renderHook(() => useAnnotations(false, identity));
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(fetchMock).not.toHaveBeenCalled();
    expect(result.current.items).toHaveLength(0);
  });

  it("create appends the created item on success", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockResolvedValueOnce(jsonResponse(item({ id: "a2" })));
    const { result } = renderHook(() => useAnnotations(true, identity));
    await waitFor(() => expect(result.current.loading).toBe(false));

    let saved = false;
    await act(async () => {
      saved = await result.current.create(
        {
          quote: { exact: "使用 SQLite", prefix: "", suffix: "" },
          position: { start: 0, end: 9 },
          version: 1,
        },
        "评论",
      );
    });
    expect(saved).toBe(true);
    expect(result.current.items.map((entry) => entry.id)).toEqual(["a2"]);
    const call = fetchMock.mock.calls[1];
    expect(String(call?.[0])).toBe("/api/annotations");
    expect(String(call?.[1]?.method)).toBe("POST");
  });

  it("create surfaces 409 as a document-changed message and keeps the list", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockResolvedValueOnce(jsonResponse({ error: "document_changed" }, 409));
    const { result } = renderHook(() => useAnnotations(true, identity));
    await waitFor(() => expect(result.current.loading).toBe(false));

    let saved = false;
    await act(async () => {
      saved = await result.current.create(
        {
          quote: { exact: "x", prefix: "", suffix: "" },
          position: { start: 0, end: 1 },
          version: 1,
        },
        "",
      );
    });
    expect(saved).toBe(false);
    expect(result.current.error).toContain("文档已经更新");
  });

  it("remove drops the item on success and reports failures", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [item()] }))
      .mockResolvedValueOnce(jsonResponse({ ok: true }));
    const { result } = renderHook(() => useAnnotations(true, identity));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    await act(async () => {
      await result.current.remove("a1");
    });
    expect(result.current.items).toHaveLength(0);

    fetchMock.mockResolvedValueOnce(
      jsonResponse({ error: "forbidden" }, 403),
    );
    await act(async () => {
      await result.current.remove("a1");
    });
    expect(result.current.error).toContain("权限");
  });

  it("reanchor replaces the stored selector", async () => {
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [item()] }))
      .mockResolvedValueOnce(
        jsonResponse(
          item({
            selector: {
              quote: { exact: "改用", prefix: "", suffix: "" },
              position: { start: 4, end: 6 },
              version: 1,
            },
          }),
        ),
      );
    const { result } = renderHook(() => useAnnotations(true, identity));
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    let done = false;
    await act(async () => {
      done = await result.current.reanchor("a1", {
        quote: { exact: "改用", prefix: "", suffix: "" },
        position: { start: 4, end: 6 },
        version: 1,
      });
    });
    expect(done).toBe(true);
    expect(result.current.items[0]?.selector.position.start).toBe(4);
  });

  it("select and clearError manage view state", async () => {
    fetchMock.mockResolvedValueOnce(jsonResponse({ annotations: [] }));
    const { result } = renderHook(() => useAnnotations(true, identity));
    await waitFor(() => expect(result.current.loading).toBe(false));

    act(() => result.current.select("x"));
    expect(result.current.selectedId).toBe("x");
    act(() => result.current.select(null));
    expect(result.current.selectedId).toBeNull();
    act(() => result.current.clearError());
    expect(result.current.error).toBeNull();
  });
});
