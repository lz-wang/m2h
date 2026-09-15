import { act, renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  APIError,
  type FileMetadata,
  type FileSummary,
  type PreviewAPI,
  type RootSummary,
} from "./api";
import { usePreview } from "./use-preview";

// A caller-controlled promise: tests decide exactly when (and whether) a
// fetch settles, so request ordering — the whole point of the decoupled
// loading contract — stays observable.
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason?: unknown) => void;
  const promise = new Promise<T>((res, rej) => {
    resolve = res;
    reject = rej;
  });
  return { promise, resolve, reject };
}

function rootWith(files: FileSummary[]): RootSummary[] {
  return [{ id: "r0", name: "docs", files }];
}

function createAPI() {
  return {
    listFiles: vi.fn<PreviewAPI["listFiles"]>(),
    getDocument: vi.fn<PreviewAPI["getDocument"]>(),
    getMarkdown: vi.fn<PreviewAPI["getMarkdown"]>(),
    getFileMetadata: vi.fn<PreviewAPI["getFileMetadata"]>(),
    search: vi.fn<PreviewAPI["search"]>(),
  };
}

function documentResponse(path: string, title = path) {
  return {
    path,
    title,
    html: `<p>Body of ${path}</p>`,
    frontmatter: null,
    toc: [],
  };
}

describe("usePreview decoupled loading", () => {
  beforeEach(() => {
    window.history.replaceState(null, "", "/");
  });

  it("renders a deep-linked document while the file list is still pending", async () => {
    window.history.replaceState(null, "", "/doc/guides/setup.md");
    const api = createAPI();
    api.listFiles.mockReturnValue(new Promise(() => {}));
    api.getDocument.mockResolvedValue(documentResponse("guides/setup.md"));
    const { result } = renderHook(() => usePreview(api));

    expect(result.current.phase).toBe("loading-document");
    await waitFor(() => expect(result.current.phase).toBe("ready"));
    expect(api.listFiles).toHaveBeenCalledTimes(1);
    expect(api.getDocument).toHaveBeenCalledWith(
      "guides/setup.md",
      expect.any(AbortSignal),
    );
    expect(result.current.document?.path).toBe("guides/setup.md");
    expect(result.current.filesError).toBeNull();
  });

  it("does not request a document from the workspace root before the file list answers", async () => {
    const api = createAPI();
    api.listFiles.mockReturnValue(new Promise(() => {}));
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    const { result } = renderHook(() => usePreview(api));

    await act(async () => {});
    expect(result.current.phase).toBe("loading-files");
    expect(result.current.filesLoading).toBe(true);
    expect(api.getDocument).not.toHaveBeenCalled();
  });

  it("opens the workspace default document once the file list arrives", async () => {
    const api = createAPI();
    const pending = deferred<Awaited<ReturnType<PreviewAPI["getDocument"]>>>();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([
        { path: "README.md", name: "README.md" },
        { path: "guides/setup.md", name: "setup.md" },
      ]),
    });
    api.getDocument.mockReturnValue(pending.promise);
    const { result } = renderHook(() => usePreview(api));

    await waitFor(() =>
      expect(api.getDocument).toHaveBeenCalledWith(
        "README.md",
        expect.any(AbortSignal),
      ),
    );
    expect(result.current.phase).toBe("loading-document");
    pending.resolve(documentResponse("README.md"));
    await waitFor(() => expect(result.current.phase).toBe("ready"));
    expect(result.current.files).toHaveLength(2);
  });

  it("refresh fetches the file list and the open document concurrently", async () => {
    window.history.replaceState(null, "", "/doc/README.md");
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([{ path: "README.md", name: "README.md" }]),
    });
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    const { result } = renderHook(() => usePreview(api));
    await waitFor(() => expect(result.current.phase).toBe("ready"));

    // Neither fetch settles during the assertion window: both calls must
    // already be in flight, which is the contract the old serial refresh
    // could not hold.
    api.listFiles.mockReturnValue(new Promise(() => {}));
    api.getDocument.mockReturnValue(new Promise(() => {}));
    await act(async () => {
      void result.current.refresh();
    });
    expect(api.listFiles).toHaveBeenCalledTimes(2);
    expect(api.getDocument).toHaveBeenCalledTimes(2);
  });

  it("keeps a ready document on screen when the file list fails", async () => {
    window.history.replaceState(null, "", "/doc/README.md");
    const api = createAPI();
    api.listFiles.mockRejectedValue(new Error("listing failed"));
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    const { result } = renderHook(() => usePreview(api));

    await waitFor(() => expect(result.current.phase).toBe("ready"));
    expect(result.current.filesError).not.toBeNull();
    expect(result.current.filesLoading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.document?.path).toBe("README.md");
  });

  it("reports not-found for a missing deep link while the sidebar still updates", async () => {
    window.history.replaceState(null, "", "/doc/gone.md");
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([{ path: "README.md", name: "README.md" }]),
    });
    api.getDocument.mockRejectedValue(new APIError(404, "not found"));
    const { result } = renderHook(() => usePreview(api));

    await waitFor(() => expect(result.current.phase).toBe("not-found"));
    expect(result.current.files).toHaveLength(1);
    expect(result.current.filesError).toBeNull();
  });

  it("dedupes concurrent metadata requests for one path", async () => {
    window.history.replaceState(null, "", "/doc/README.md");
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([{ path: "README.md", name: "README.md" }]),
    });
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    const pending = deferred<FileMetadata>();
    api.getFileMetadata.mockReturnValue(pending.promise);
    const { result } = renderHook(() => usePreview(api));
    await waitFor(() => expect(result.current.phase).toBe("ready"));
    expect(api.getFileMetadata).not.toHaveBeenCalled();

    await act(async () => {
      const first = result.current.loadFileMetadata("README.md");
      const second = result.current.loadFileMetadata("README.md");
      expect(api.getFileMetadata).toHaveBeenCalledTimes(1);
      pending.resolve({ title: "Meta" });
      await Promise.all([first, second]);
    });
    expect(result.current.metadata.get("README.md")).toEqual({ title: "Meta" });

    // A settled entry answers from the cache without a new request.
    await result.current.loadFileMetadata("README.md");
    expect(api.getFileMetadata).toHaveBeenCalledTimes(1);
  });

  it("drops the metadata cache whenever the file list reloads", async () => {
    window.history.replaceState(null, "", "/doc/README.md");
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([{ path: "README.md", name: "README.md" }]),
    });
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    api.getFileMetadata.mockResolvedValue({ title: "Meta" });
    const { result } = renderHook(() => usePreview(api));
    await waitFor(() => expect(result.current.phase).toBe("ready"));

    await act(async () => {
      await result.current.loadFileMetadata("README.md");
    });
    expect(result.current.metadata.get("README.md")).toEqual({ title: "Meta" });

    await act(async () => {
      await result.current.refresh();
    });
    expect(result.current.metadata.size).toBe(0);

    // The cleared cache refetches: stale metadata never survives a reload.
    await act(async () => {
      await result.current.loadFileMetadata("README.md");
    });
    expect(api.getFileMetadata).toHaveBeenCalledTimes(2);
  });

  it("keeps the in-flight dedupe entry when a stale request settles after a refresh", async () => {
    window.history.replaceState(null, "", "/doc/README.md");
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([{ path: "README.md", name: "README.md" }]),
    });
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    // Base fallback: any unexpected third request resolves visibly.
    api.getFileMetadata.mockImplementation(async () => ({ title: "P3" }));
    const { result } = renderHook(() => usePreview(api));
    await waitFor(() => expect(result.current.phase).toBe("ready"));

    // P1 goes in flight, then a refresh drops the cache while it is pending.
    const stale = deferred<FileMetadata>();
    api.getFileMetadata.mockReturnValueOnce(stale.promise);
    await act(async () => {
      void result.current.loadFileMetadata("README.md");
    });
    await act(async () => {
      await result.current.refresh();
    });

    // P2 is the fresh in-flight request after the reload.
    const fresh = deferred<FileMetadata>();
    api.getFileMetadata.mockReturnValueOnce(fresh.promise);
    await act(async () => {
      void result.current.loadFileMetadata("README.md");
    });
    expect(api.getFileMetadata).toHaveBeenCalledTimes(2);

    // P1 settles late. Its generation guard must keep it from writing stale
    // metadata — and it must not evict P2's dedupe entry on its way out.
    await act(async () => {
      stale.resolve({ title: "Stale" });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(result.current.metadata.has("README.md")).toBe(false);

    // The next hover must reuse the still-in-flight P2, not issue a P3.
    await act(async () => {
      void result.current.loadFileMetadata("README.md");
    });
    expect(api.getFileMetadata).toHaveBeenCalledTimes(2);

    await act(async () => {
      fresh.resolve({ title: "Fresh" });
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(result.current.metadata.get("README.md")).toEqual({
      title: "Fresh",
    });
  });
});
