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
  updateAnnotationComment,
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
    onCreated?: (id: string) => void,
  ) => Promise<boolean>;
  remove: (ids: string | string[]) => Promise<void>;
  // Persists a human-confirmed re-anchor (new selector) for one item.
  reanchor: (id: string, selector: AnnotationSelectorData) => Promise<boolean>;
  editComment: (id: string, comment: string) => Promise<boolean>;
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
  const pendingWrites = useRef<{
    identity: AnnotationIdentity;
    changes: Map<string, AnnotationItem | null>;
  } | null>(null);
  // The identity every in-flight write was started against. Writes cannot
  // be aborted like the listing fetch — the server must see them through —
  // but their responses must never update a different document's state:
  // a write resolving after a document switch is dropped on arrival. The
  // check is by document (sameDocument), not object identity: switching
  // away and back produces a fresh identity object for the same document,
  // whose late responses still belong.
  const identityRef = useRef<AnnotationIdentity | null>(identity);
  identityRef.current = identity;

  useEffect(() => {
    requestRef.current?.abort();
    if (!enabled || identity === null) {
      pendingWrites.current = null;
      setState({ items: [], loading: false, error: null, selectedId: null });
      return;
    }
    const controller = new AbortController();
    requestRef.current = controller;
    const writes = {
      identity,
      changes: new Map<string, AnnotationItem | null>(),
    };
    pendingWrites.current = writes;
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
        const merged = new Map(items.map((item) => [item.id, item]));
        for (const [id, item] of writes.changes) {
          if (item === null) merged.delete(id);
          else merged.set(id, item);
        }
        if (pendingWrites.current === writes) pendingWrites.current = null;
        setState((current) => ({
          ...current,
          items: [...merged.values()],
          loading: false,
        }));
      } catch (error) {
        if (controller.signal.aborted || isAbort(error)) {
          return;
        }
        if (pendingWrites.current === writes) pendingWrites.current = null;
        setState((current) => ({
          ...current,
          loading: false,
          error: errorMessage(error, "无法加载标注"),
        }));
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
      onCreated?: (id: string) => void,
    ): Promise<boolean> => {
      if (identity === null) {
        return false;
      }
      const writtenFor = identity;
      try {
        const created = await createAnnotation(
          writtenFor.documentRef,
          writtenFor.documentHash,
          selector,
          comment,
        );
        if (sameDocument(identityRef.current, writtenFor)) {
          if (
            pendingWrites.current &&
            sameDocument(pendingWrites.current.identity, writtenFor)
          ) {
            pendingWrites.current.changes.set(created.id, created);
          }
          setState((current) => ({
            ...current,
            items: current.items.some((item) => item.id === created.id)
              ? current.items
              : [...current.items, created],
          }));
          onCreated?.(created.id);
        }
        return true;
      } catch (error) {
        if (sameDocument(identityRef.current, writtenFor)) {
          setState((current) => ({
            ...current,
            error: errorMessage(error, "保存标注失败"),
          }));
        }
        return false;
      }
    },
    [identity],
  );

  const remove = useCallback(async (ids: string | string[]): Promise<void> => {
    const writtenFor = identityRef.current;
    if (writtenFor === null) return;
    for (const id of typeof ids === "string" ? [ids] : ids) {
      try {
        await deleteAnnotation(id);
        if (sameDocumentPath(identityRef.current, writtenFor)) {
          if (
            pendingWrites.current &&
            sameDocumentPath(pendingWrites.current.identity, writtenFor)
          ) {
            pendingWrites.current.changes.set(id, null);
          }
          setState((current) => ({
            ...current,
            items: current.items.filter((item) => item.id !== id),
            selectedId: current.selectedId === id ? null : current.selectedId,
          }));
        }
      } catch (error) {
        if (sameDocumentPath(identityRef.current, writtenFor)) {
          setState((current) => ({
            ...current,
            error: errorMessage(error, "删除标注失败"),
          }));
        }
      }
    }
  }, []);

  const reanchor = useCallback(
    async (id: string, selector: AnnotationSelectorData): Promise<boolean> => {
      if (identity === null) {
        return false;
      }
      const writtenFor = identity;
      try {
        const updated = await reanchorAnnotation(
          id,
          writtenFor.documentHash,
          selector,
        );
        if (sameDocument(identityRef.current, writtenFor)) {
          if (
            pendingWrites.current &&
            sameDocument(pendingWrites.current.identity, writtenFor)
          ) {
            pendingWrites.current.changes.set(id, updated);
          }
          setState((current) => ({
            ...current,
            items: current.items.map((item) =>
              item.id === id ? { ...updated, mine: item.mine } : item,
            ),
          }));
        }
        return true;
      } catch (error) {
        if (sameDocument(identityRef.current, writtenFor)) {
          setState((current) => ({
            ...current,
            error: errorMessage(error, "更新标注位置失败"),
          }));
        }
        return false;
      }
    },
    [identity],
  );

  const select = useCallback((id: string | null) => {
    setState((current) => ({ ...current, selectedId: id }));
  }, []);

  const editComment = useCallback(
    async (id: string, comment: string): Promise<boolean> => {
      const writtenFor = identityRef.current;
      if (writtenFor === null) return false;
      try {
        const updated = await updateAnnotationComment(id, comment);
        if (sameDocument(identityRef.current, writtenFor)) {
          if (
            pendingWrites.current &&
            sameDocument(pendingWrites.current.identity, writtenFor)
          )
            pendingWrites.current.changes.set(id, updated);
          setState((current) => ({
            ...current,
            items: current.items.map((item) =>
              item.id === id ? updated : item,
            ),
          }));
        }
        return true;
      } catch (error) {
        if (sameDocument(identityRef.current, writtenFor))
          setState((current) => ({
            ...current,
            error: errorMessage(error, "更新评论失败"),
          }));
        return false;
      }
    },
    [],
  );

  const clearError = useCallback(() => {
    setState((current) => ({ ...current, error: null }));
  }, []);

  return {
    ...state,
    identity,
    create,
    remove,
    reanchor,
    editComment,
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

// sameDocument reports whether two identities address the same document
// at the same content version — the equivalence late write responses are
// judged by.
function sameDocument(
  left: AnnotationIdentity | null,
  right: AnnotationIdentity | null,
): boolean {
  return (
    sameDocumentPath(left, right) && left?.documentHash === right?.documentHash
  );
}

// Deleting an annotation ID applies across versions of its document.
function sameDocumentPath(
  left: AnnotationIdentity | null,
  right: AnnotationIdentity | null,
): boolean {
  return (
    left !== null &&
    right !== null &&
    left.path === right.path &&
    left.documentRef.rootId === right.documentRef.rootId
  );
}
