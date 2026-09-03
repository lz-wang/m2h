import { act, renderHook } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { PreviewAPI, SearchResponse } from "./api";
import { useSearch } from "./use-search";

// The search state machine is timing-sensitive: fake timers pin the debounce
// window and manually resolved promises let the tests interleave responses
// in the exact order the races live in.

function deferred<T>(value: T): {
  promise: Promise<T>;
  resolve(): void;
} {
  let resolveFn: () => void = () => {};
  const promise = new Promise<T>((resolve) => {
    resolveFn = () => resolve(value);
  });
  return { promise, resolve: () => resolveFn() };
}

function createAPIMock(): {
  api: PreviewAPI;
  calls: Array<{ query: string; signal: AbortSignal }>;
} {
  const calls: Array<{ query: string; signal: AbortSignal }> = [];
  const api: PreviewAPI = {
    listFiles: vi.fn(),
    getDocument: vi.fn(),
    getMarkdown: vi.fn(),
    search: vi.fn((_query: string, signal?: AbortSignal) => {
      const response: SearchResponse = { query: "", results: [] };
      calls.push({
        query: _query,
        signal: signal ?? new AbortController().signal,
      });
      return Promise.resolve(response);
    }),
  };
  return { api, calls };
}

async function typeQuery(
  result: { current: ReturnType<typeof useSearch> },
  query: string,
): Promise<void> {
  await act(async () => {
    result.current.setQuery(query);
  });
}

describe("useSearch", () => {
  beforeEach(() => {
    // Only the timers the debounce uses: the shared setup pins
    // requestAnimationFrame read-only, so the default all-timers scope
    // cannot install here.
    vi.useFakeTimers({ toFake: ["setTimeout", "clearTimeout"] });
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it("starts idle with no results", () => {
    const { api } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    expect(result.current.phase).toBe("idle");
    expect(result.current.query).toBe("");
    expect(result.current.results).toEqual([]);
    expect(result.current.error).toBeNull();
  });

  it("does not request for a single rune", async () => {
    const { api, calls } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "图");
    await act(async () => {
      vi.advanceTimersByTime(500);
    });

    expect(calls).toHaveLength(0);
    expect(result.current.phase).toBe("idle");
    expect(result.current.results).toEqual([]);
  });

  it("debounces and searches once typing settles", async () => {
    const { api, calls } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "goldmark");
    // Feedback starts with typing; the request waits for the debounce.
    expect(result.current.phase).toBe("searching");
    expect(calls).toHaveLength(0);

    await act(async () => {
      vi.advanceTimersByTime(199);
    });
    expect(calls).toHaveLength(0);

    await act(async () => {
      vi.advanceTimersByTime(1);
    });
    expect(calls).toHaveLength(1);
    expect(calls[0]?.query).toBe("goldmark");
  });

  it("searches the trimmed query", async () => {
    const { api, calls } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "  goldmark  ");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });

    expect(calls[0]?.query).toBe("goldmark");
  });

  it("reschedules while typing so only the settled query fires", async () => {
    const { api, calls } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "go");
    await act(async () => {
      vi.advanceTimersByTime(100);
    });
    await typeQuery(result, "goldmark parser");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });

    expect(calls).toHaveLength(1);
    expect(calls[0]?.query).toBe("goldmark parser");
  });

  it("aborts the previous request when a new one fires", async () => {
    const { api, calls } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "first");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    await typeQuery(result, "second");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });

    expect(calls).toHaveLength(2);
    expect(calls[0]?.signal.aborted).toBe(true);
    expect(calls[1]?.signal.aborted).toBe(false);
  });

  it("ignores a stale response that lands after a newer one", async () => {
    const { api } = createAPIMock();
    // Both requests park until the test releases them, in the order the
    // race actually happens: the newer one resolves first.
    const first = deferred<SearchResponse>({
      query: "first",
      results: [{ path: "old.md", title: "Old" }],
    });
    const second = deferred<SearchResponse>({
      query: "second",
      results: [{ path: "new.md", title: "New" }],
    });
    vi.mocked(api.search)
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise);

    const { result } = renderHook(() => useSearch(api));
    await typeQuery(result, "first");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    await typeQuery(result, "second");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });

    await act(async () => {
      second.resolve();
      await Promise.resolve();
    });
    expect(result.current.phase).toBe("ready");
    expect(result.current.results).toEqual([{ path: "new.md", title: "New" }]);

    // The late first response must not overwrite the newer results.
    await act(async () => {
      first.resolve();
      await Promise.resolve();
    });
    expect(result.current.results).toEqual([{ path: "new.md", title: "New" }]);
    expect(result.current.phase).toBe("ready");
  });

  it("reports errors and recovers on the next query", async () => {
    const { api } = createAPIMock();
    vi.mocked(api.search)
      .mockImplementationOnce(() => Promise.reject(new Error("网络错误")))
      .mockImplementationOnce(() =>
        Promise.resolve({ query: "next", results: [] }),
      );
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "bad");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current.phase).toBe("error");
    expect(result.current.error).toBe("网络错误");
    expect(result.current.results).toEqual([]);

    await typeQuery(result, "next");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current.phase).toBe("ready");
    expect(result.current.error).toBeNull();
  });

  it("reset returns to idle and clears everything", async () => {
    const { api } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "reset-me");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current.phase).toBe("ready");

    await act(async () => {
      result.current.reset();
    });
    expect(result.current.query).toBe("");
    expect(result.current.results).toEqual([]);
    expect(result.current.error).toBeNull();
    expect(result.current.phase).toBe("idle");
  });

  it("clearing the query aborts the in-flight request", async () => {
    const { api, calls } = createAPIMock();
    const { result } = renderHook(() => useSearch(api));

    await typeQuery(result, "in-flight");
    await act(async () => {
      vi.advanceTimersByTime(200);
    });
    expect(calls).toHaveLength(1);

    await typeQuery(result, "");
    expect(calls[0]?.signal.aborted).toBe(true);
    expect(result.current.phase).toBe("idle");
  });
});
