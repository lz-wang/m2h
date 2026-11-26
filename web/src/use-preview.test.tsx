import {
  act,
  fireEvent,
  render,
  renderHook,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  APIError,
  type FileMetadata,
  type FileSummary,
  type PreviewAPI,
  type RootSummary,
} from "./api";
import { HomePage } from "./components/home-page";
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
    getRecent: vi
      .fn<PreviewAPI["getRecent"]>()
      .mockResolvedValue({ created: [], updated: [] }),
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

  it("writes the annotation panel state without changing the current view settings or anchor", async () => {
    window.history.replaceState(
      null,
      "",
      "/doc/README.md?mode=dark&width=wide&toc=false#part",
    );
    const api = createAPI();
    api.listFiles.mockReturnValue(new Promise(() => {}));
    api.getDocument.mockResolvedValue(documentResponse("README.md"));
    const { result } = renderHook(() => usePreview(api));
    await waitFor(() => expect(result.current.phase).toBe("ready"));
    const historyLength = window.history.length;

    act(() => result.current.setAnnotationsOpen(true));
    expect(result.current.annotationsOpen).toBe(true);
    expect(window.location.search).toBe(
      "?mode=dark&width=wide&toc=false&annotations=true",
    );
    expect(window.location.hash).toBe("#part");
    expect(window.history.length).toBe(historyLength);

    act(() => result.current.setAnnotationsOpen(false));
    expect(result.current.annotationsOpen).toBe(false);
    expect(window.location.search).toBe("?mode=dark&width=wide&toc=false");
    expect(window.location.hash).toBe("#part");
    expect(api.getDocument).toHaveBeenCalledTimes(1);
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

  it("enters the directory home once the file list arrives", async () => {
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([
        { path: "README.md", name: "README.md" },
        { path: "guides/setup.md", name: "setup.md" },
      ]),
    });
    const { result } = renderHook(() => usePreview(api));

    await waitFor(() => expect(result.current.phase).toBe("home"));
    expect(result.current.files).toHaveLength(2);
    expect(api.getDocument).not.toHaveBeenCalled();
    expect(window.location.pathname).toBe("/");
  });

  it("refreshes both recent lists when the home file listing reloads", async () => {
    const api = createAPI();
    const files = [{ path: "README.md", name: "README.md" }];
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith(files),
    });
    const entry = {
      path: "README.md",
      title: "Old title",
      date: "2026-11-19",
      dateSource: "frontmatter",
    } as const;
    const stale = deferred<Awaited<ReturnType<PreviewAPI["getRecent"]>>>();
    const latest = { ...entry, title: "New title", date: "2026-11-20" };
    api.getRecent
      .mockResolvedValueOnce({ created: [entry], updated: [entry] })
      .mockReturnValueOnce(stale.promise)
      .mockResolvedValueOnce({ created: [latest], updated: [latest] });
    const onRecentSettled = vi.fn();

    function PreviewHarness() {
      const preview = usePreview(api);
      return preview.phase === "home" ? (
        <>
          <button type="button" onClick={() => void preview.refresh()}>
            Refresh
          </button>
          <HomePage
            getRecent={api.getRecent}
            revision={preview.homeRequest.revision}
            forceRefresh={preview.homeRequest.forceRefresh}
            onRecentSettled={onRecentSettled}
            fileCount={preview.files.length}
            roots={preview.roots}
            options={preview}
            onSelect={preview.select}
          />
        </>
      ) : null;
    }

    render(<PreviewHarness />);
    const created = within(
      await screen.findByRole("region", { name: "最新创建" }),
    );
    expect(created.getByRole("link", { name: /Old title/ })).toBeTruthy();
    expect(api.getRecent).toHaveBeenLastCalledWith(
      expect.any(AbortSignal),
      false,
    );
    const refresh = screen.getByRole("button", { name: "Refresh" });
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([...files, { path: "new.md", name: "new.md" }]),
    });
    fireEvent.click(refresh);
    await waitFor(() => expect(api.getRecent).toHaveBeenCalledTimes(2));
    expect(api.getRecent).toHaveBeenLastCalledWith(
      expect.any(AbortSignal),
      true,
    );
    fireEvent.click(refresh);
    const updated = within(
      await screen.findByRole("region", { name: "最近更新" }),
    );
    expect(updated.getByRole("link", { name: /New title/ })).toBeTruthy();
    expect(
      within(screen.getByRole("region", { name: "最新创建" })).getByRole(
        "link",
        { name: /New title/ },
      ),
    ).toBeTruthy();
    expect(screen.getByText("2")).toBeTruthy();
    expect(api.getRecent).toHaveBeenCalledTimes(3);
    expect(api.getRecent.mock.calls[1]?.[0]?.aborted).toBe(true);
    await act(async () =>
      stale.resolve({ created: [entry], updated: [entry] }),
    );
    expect(screen.queryByRole("link", { name: /Old title/ })).toBeNull();
  });

  it("aborts an open document request when returning home", async () => {
    window.history.replaceState(null, "", "/?mode=dark&width=wide&toc=false");
    const api = createAPI();
    api.listFiles.mockResolvedValue({
      kind: "directory",
      version: "test",
      roots: rootWith([{ path: "README.md", name: "README.md" }]),
    });
    const pending = deferred<Awaited<ReturnType<PreviewAPI["getDocument"]>>>();
    api.getDocument.mockReturnValue(pending.promise);
    const { result } = renderHook(() => usePreview(api));

    await waitFor(() => expect(result.current.phase).toBe("home"));
    act(() => {
      void result.current.select("README.md", "#heading");
    });
    const signal = api.getDocument.mock.calls[0]?.[1];
    expect(result.current.phase).toBe("loading-document");
    act(() => result.current.goHome());

    expect(signal?.aborted).toBe(true);
    expect(
      window.location.pathname + window.location.search + window.location.hash,
    ).toBe("/?mode=dark&width=wide&toc=false");

    await act(async () => pending.resolve(documentResponse("README.md")));
    expect(result.current.phase).toBe("home");
    expect(result.current.document).toBeNull();
    expect(result.current.selectedPath).toBeNull();
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
