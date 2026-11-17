import { useCallback, useEffect, useRef, useState } from "react";

import {
  type AnnotationItem,
  type AnnotationSelectorData,
  APIError,
  createAnnotation,
  type DocumentRef,
  deleteAnnotation,
  listAnnotations,
  reanchorAnnotation,
} from "../api";

// AnnotationState is the per-document annotation view: items are the
// stored list, loading/error describe the current fetch, selectedId marks
// the annotation the UI should scroll to or flash.
export interface AnnotationState {
  items: AnnotationItem[];
  loading: boolean;
  error: string | null;
  selectedId: string | null;
}

export interface UseAnnotationsResult extends AnnotationState {
  // The current document identity the list belongs to; null while no
  // annotatable document is open (default mode, loading, or anonymous).
  identity: AnnotationIdentity | null;
  // Saves a new annotation against the currently loaded document. Returns
  // false and fills `error` on failure — the caller keeps its selection so
  // a retry loses nothing.
  create: (
    selector: AnnotationSelectorData,
    comment: string,
  ) => Promise<boolean>;
  remove: (id: string) => Promise<void>;
  // Persists a human-confirmed re-anchor (new selector) for one item.
  reanchor: (id: string, selector: AnnotationSelectorData) => Promise<boolean>;
  select: (id: string | null) => void;
  clearError: () => void;
}

export interface AnnotationIdentity {
  // The browser-facing virtual path (what /doc/ and the sidebar use).
  path: string;
  documentRef: DocumentRef;
  documentHash: string;
}

// useAnnotations owns the annotation list of the selected document. It is
// deliberately separate from usePreview: the preview loads documents, this
// hook loads — and mutates — the annotations attached to whichever
// document the preview settled on. Requests are aborted on every document
// switch, so a slow response can never surface in the next document.
export function useAnnotations(
  enabled: boolean,
  identity: AnnotationIdentity | null,
): UseAnnotationsResult {
  const [state, setState] = useState<AnnotationState>({
    items: [],
    loading: false,
    error: null,
    selectedId: null,
  });

  const requestRef = useRef<AbortController | null>(null);
  useEffect(() => {
    requestRef.current?.abort();
    if (!enabled || identity === null) {
      setState({ items: [], loading: false, error: null, selectedId: null });
      return;
    }
    const controller = new AbortController();
    requestRef.current = controller;
    // The identity changed: the previous document's annotations are stale
    // immediately, not when the fetch resolves — old highlights must never
    // ride along onto the new document.
    setState({ items: [], loading: true, error: null, selectedId: null });
    void (async () => {
      try {
        const items = await listAnnotations(identity.path, controller.signal);
        if (controller.signal.aborted) {
          return;
        }
        setState({ items, loading: false, error: null, selectedId: null });
      } catch (error) {
        if (controller.signal.aborted || isAbort(error)) {
          return;
        }
        setState({
          items: [],
          loading: false,
          error: errorMessage(error, "无法加载标注"),
          selectedId: null,
        });
      }
    })();
    return () => {
      controller.abort();
    };
  }, [enabled, identity]);

  const create = useCallback(
    async (
      selector: AnnotationSelectorData,
      comment: string,
    ): Promise<boolean> => {
      if (identity === null) {
        return false;
      }
      try {
        const created = await createAnnotation(
          identity.documentRef,
          identity.documentHash,
          selector,
          comment,
        );
        setState((current) => ({
          ...current,
          items: [...current.items, created],
        }));
        return true;
      } catch (error) {
        setState((current) => ({
          ...current,
          error: errorMessage(error, "保存标注失败"),
        }));
        return false;
      }
    },
    [identity],
  );

  const remove = useCallback(async (id: string): Promise<void> => {
    try {
      await deleteAnnotation(id);
      setState((current) => ({
        ...current,
        items: current.items.filter((item) => item.id !== id),
        selectedId: current.selectedId === id ? null : current.selectedId,
      }));
    } catch (error) {
      setState((current) => ({
        ...current,
        error: errorMessage(error, "删除标注失败"),
      }));
    }
  }, []);

  const reanchor = useCallback(
    async (id: string, selector: AnnotationSelectorData): Promise<boolean> => {
      try {
        const updated = await reanchorAnnotation(id, selector);
        setState((current) => ({
          ...current,
          items: current.items.map((item) =>
            item.id === id ? { ...updated, mine: item.mine } : item,
          ),
        }));
        return true;
      } catch (error) {
        setState((current) => ({
          ...current,
          error: errorMessage(error, "更新标注位置失败"),
        }));
        return false;
      }
    },
    [],
  );

  const select = useCallback((id: string | null) => {
    setState((current) => ({ ...current, selectedId: id }));
  }, []);

  const clearError = useCallback(() => {
    setState((current) => ({ ...current, error: null }));
  }, []);

  return {
    ...state,
    identity,
    create,
    remove,
    reanchor,
    select,
    clearError,
  };
}

function errorMessage(error: unknown, fallback: string): string {
  if (error instanceof APIError) {
    if (error.status === 409) {
      return "文档已经更新，请重新加载后再标注。";
    }
    if (error.status === 403) {
      return "没有权限执行此操作。";
    }
    if (error.status === 429) {
      return "操作过于频繁，请稍后再试。";
    }
  }
  return fallback;
}

function isAbort(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}
