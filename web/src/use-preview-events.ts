import { useEffect } from "react";

/**
 * Subscribes to the preview server-sent events stream and invokes the callback
 * whenever the open document changes on disk. The callback is the only
 * dependency: pass a stable one (e.g. a useCallback) so the EventSource is not
 * reconnected on every render.
 */
export function usePreviewEvents(onDocumentChanged: () => void): void {
  useEffect(() => {
    const source = new EventSource("/api/events");
    source.addEventListener("document-changed", () => onDocumentChanged());
    return () => {
      source.close();
    };
  }, [onDocumentChanged]);
}
