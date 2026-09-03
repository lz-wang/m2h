import { useCallback, useEffect, useRef, useState } from "react";

import { browserAPI, type PreviewAPI, type SearchResult } from "./api";

// The full-text search state machine: a workspace-level transient state,
// deliberately separate from usePreview (which owns documents, history,
// theme and layout). Debounced queries, request cancellation and generation
// guards live here so the dialog component stays presentational.
export type SearchPhase = "idle" | "searching" | "ready" | "error";

// Keystrokes settle for this long before a request goes out.
const searchDebounceMs = 200;
// The server accepts a single rune; the client only auto-sends two or more
// so one stray CJK character does not scan the whole workspace. A one-rune
// query can still be sent deliberately by pressing Enter — the API keeps
// the full contract.
const minAutoSearchRunes = 2;

export interface UseSearchState {
  query: string;
  setQuery(query: string): void;
  results: SearchResult[];
  phase: SearchPhase;
  error: string | null;
  // Resets everything back to idle — used when the search dialog closes.
  reset(): void;
}

function runeCount(value: string): number {
  return [...value].length;
}

function isAbortError(reason: unknown): boolean {
  return reason instanceof DOMException && reason.name === "AbortError";
}

export function useSearch(api: PreviewAPI = browserAPI): UseSearchState {
  const [query, setQueryState] = useState("");
  const [results, setResults] = useState<SearchResult[]>([]);
  const [phase, setPhase] = useState<SearchPhase>("idle");
  const [error, setError] = useState<string | null>(null);
  const controller = useRef<AbortController | null>(null);
  // Only the newest request may commit its response. AbortController alone
  // is not enough: a fetch that already resolved before the abort lands
  // would still overwrite fresher results, exactly like usePreview's
  // document requests.
  const generation = useRef(0);

  const runSearch = useCallback(
    async (candidate: string) => {
      controller.current?.abort();
      const next = new AbortController();
      controller.current = next;
      generation.current += 1;
      const request = generation.current;
      try {
        const response = await api.search(candidate, next.signal);
        if (request !== generation.current || next.signal.aborted) {
          return;
        }
        setResults(response.results);
        setError(null);
        setPhase("ready");
      } catch (reason: unknown) {
        if (next.signal.aborted || isAbortError(reason)) {
          return;
        }
        if (request !== generation.current) {
          return;
        }
        setResults([]);
        setError(reason instanceof Error ? reason.message : "搜索失败");
        setPhase("error");
      }
    },
    [api],
  );

  const invalidate = useCallback(() => {
    generation.current += 1;
    controller.current?.abort();
    controller.current = null;
  }, []);

  useEffect(() => {
    const trimmed = query.trim();
    if (runeCount(trimmed) < minAutoSearchRunes) {
      invalidate();
      setResults([]);
      setError(null);
      setPhase("idle");
      return;
    }
    // The spinner starts with typing, not with the request: the debounce is
    // invisible latency, feedback should not be.
    setPhase("searching");
    const timer = window.setTimeout(() => {
      void runSearch(trimmed);
    }, searchDebounceMs);
    return () => {
      window.clearTimeout(timer);
    };
  }, [query, invalidate, runSearch]);

  // Unmount invalidates the in-flight request; the effect above covers the
  // query-change paths.
  useEffect(
    () => () => {
      invalidate();
    },
    [invalidate],
  );

  const setQuery = useCallback((next: string) => {
    setQueryState(next);
  }, []);

  const reset = useCallback(() => {
    invalidate();
    setQueryState("");
    setResults([]);
    setError(null);
    setPhase("idle");
  }, [invalidate]);

  return { query, setQuery, results, phase, error, reset };
}
