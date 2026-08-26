import { useEffect } from "react";

/**
 * Subscribes to the document server's event stream and invokes the callback
 * whenever the served workspace changes on disk — an edited, added, or removed
 * file in any root. The callback is the only dependency: pass a stable one
 * (e.g. a useCallback) so the EventSource is not reconnected on every render.
 */
export function useWorkspaceEvents(onWorkspaceChanged: () => void): void {
  useEffect(() => {
    const source = new EventSource("/api/events");
    source.addEventListener("workspace-changed", () => onWorkspaceChanged());
    return () => {
      source.close();
    };
  }, [onWorkspaceChanged]);
}
