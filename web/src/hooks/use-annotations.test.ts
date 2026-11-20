import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { AnnotationItem } from "../api";
import { type AnnotationIdentity, useAnnotations } from "./use-annotations";

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
      const outcome = await result.current.remove("a1");
      expect(outcome).toEqual({ deletedIds: ["a1"], failures: [] });
    });
    expect(result.current.items).toHaveLength(0);

    fetchMock.mockResolvedValueOnce(jsonResponse({ error: "forbidden" }, 403));
    await act(async () => {
      const outcome = await result.current.remove("a1");
      expect(outcome).toEqual({
        deletedIds: [],
        failures: [{ id: "a1", message: "没有权限执行此操作。" }],
      });
    });
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
    // The re-anchor states the document version it was made on.
    const call = fetchMock.mock.calls.at(-1);
    expect(String(call?.[0])).toBe("/api/annotations/a1/anchor");
    const body = JSON.parse(String(call?.[1]?.body)) as {
      documentHash: string;
    };
    expect(body.documentHash).toBe("sha256:abc");
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

  it("a create resolving after a document switch never touches the new document", async () => {
    let resolveCreate: (value: Response) => void = () => {};
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveCreate = resolve;
          }),
      )
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }));
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.loading).toBe(false));

    let createDone: Promise<boolean> | null = null;
    act(() => {
      createDone = result.current.create(
        {
          quote: { exact: "旧文档的引用", prefix: "", suffix: "" },
          position: { start: 0, end: 7 },
          version: 1,
        },
        "",
      );
    });
    // The user switches documents while the create is in flight.
    rerender({
      identity: { ...identity, path: "other.md", documentHash: "sha256:def" },
    });
    await act(async () => {
      resolveCreate(jsonResponse(item({ id: "stale" })));
      await createDone;
    });
    expect(createDone).not.toBeNull();
    await createDone;
    // The late response belongs to the previous document: the new
    // document's list must stay untouched.
    expect(result.current.items).toHaveLength(0);
  });

  it("a late response still applies after switching AWAY and BACK to the same document", async () => {
    // Same path, same content hash — a fresh identity object, but the
    // same document. Dropping the response on object identity alone
    // would hide a successfully saved annotation until the next reload.
    let resolveCreate: (value: Response) => void = () => {};
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveCreate = resolve;
          }),
      )
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }));
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.loading).toBe(false));

    let createDone: Promise<boolean> | null = null;
    act(() => {
      createDone = result.current.create(
        {
          quote: { exact: "同一段的引用", prefix: "", suffix: "" },
          position: { start: 0, end: 7 },
          version: 1,
        },
        "",
      );
    });
    // Away to another document, then back to the same one (new object,
    // same path + hash).
    rerender({
      identity: { ...identity, path: "other.md", documentHash: "sha256:def" },
    });
    rerender({ identity: { ...identity } });
    await act(async () => {
      resolveCreate(jsonResponse(item({ id: "applied" })));
      await createDone;
    });
    expect(createDone).not.toBeNull();
    await createDone;
    expect(result.current.items.map((entry) => entry.id)).toEqual(["applied"]);
  });

  it("does not duplicate a creation already returned by a reload", async () => {
    let resolveCreate: (value: Response) => void = () => {};
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveCreate = resolve;
          }),
      )
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockResolvedValueOnce(jsonResponse({ annotations: [item()] }));
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.loading).toBe(false));
    let pending: Promise<boolean> | undefined;
    act(() => {
      pending = result.current.create(item().selector, "评论");
    });
    rerender({ identity: { ...identity, path: "other.md" } });
    rerender({ identity: { ...identity } });
    await waitFor(() => expect(result.current.items).toHaveLength(1));
    await act(async () => {
      resolveCreate(jsonResponse(item()));
      await pending;
    });
    expect(result.current.items.map((entry) => entry.id)).toEqual(["a1"]);
  });

  it("retains a creation when an older list response arrives afterward", async () => {
    let resolveCreate: (value: Response) => void = () => {};
    let resolveList: (value: Response) => void = () => {};
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveCreate = resolve;
          }),
      )
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveList = resolve;
          }),
      );
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.loading).toBe(false));
    let pending: Promise<boolean> | undefined;
    act(() => {
      pending = result.current.create(item().selector, "评论");
    });
    rerender({ identity: { ...identity, path: "other.md" } });
    rerender({ identity: { ...identity } });
    await act(async () => {
      resolveCreate(jsonResponse(item()));
      await pending;
    });
    await act(async () => {
      resolveList(jsonResponse({ annotations: [] }));
    });
    expect(result.current.items.map((entry) => entry.id)).toEqual(["a1"]);
    expect(result.current.loading).toBe(false);
  });

  it("a failed remove after a document switch does not surface on the new document", async () => {
    let rejectRemove: (reason?: unknown) => void = () => {};
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [item()] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((_resolve, reject) => {
            rejectRemove = reject;
          }),
      )
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }));
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    let removeDone: ReturnType<typeof result.current.remove> | null = null;
    act(() => {
      removeDone = result.current.remove("a1");
    });
    rerender({ identity: { ...identity, path: "other.md" } });
    await act(async () => {
      rejectRemove(new Error("network died"));
      await removeDone;
    });
    expect(result.current.error).toBeNull();
    expect(result.current.loading).toBe(false);
  });

  it("a reanchor resolving after a document switch never mutates the new list", async () => {
    let resolveReanchor: (value: Response) => void = () => {};
    fetchMock
      .mockResolvedValueOnce(jsonResponse({ annotations: [item()] }))
      .mockImplementationOnce(
        () =>
          new Promise<Response>((resolve) => {
            resolveReanchor = resolve;
          }),
      )
      .mockResolvedValueOnce(jsonResponse({ annotations: [] }));
    const { result, rerender } = renderHook(
      ({ identity: current }) => useAnnotations(true, current),
      { initialProps: { identity } },
    );
    await waitFor(() => expect(result.current.items).toHaveLength(1));

    let done: Promise<boolean> | null = null;
    act(() => {
      done = result.current.reanchor("a1", {
        quote: { exact: "改用", prefix: "", suffix: "" },
        position: { start: 4, end: 6 },
        version: 1,
      });
    });
    rerender({
      identity: { ...identity, path: "other.md", documentHash: "sha256:def" },
    });
    await act(async () => {
      resolveReanchor(
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
      await done;
    });
    expect(done).not.toBeNull();
    await done;
    expect(result.current.items).toHaveLength(0);
    expect(result.current.error).toBeNull();
  });
});
