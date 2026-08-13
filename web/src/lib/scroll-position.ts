// Persists the reader's vertical scroll position per document so a page refresh
// can return to the exact pixel the reader left off at — more precise than the
// URL hash, which only records the enclosing section.
//
// Lives in sessionStorage (per tab, cleared when the tab closes) and is never
// encoded into the URL, so a shareable link still targets a section while the
// current tab remembers the precise spot. A saved position of 0 (top) is
// treated as absent: the default already lands at the top.

const prefix = "m2h.scroll.";

function key(path: string): string {
  return `${prefix}${path}`;
}

export function saveScrollPosition(path: string, scrollTop: number): void {
  try {
    window.sessionStorage.setItem(key(path), String(scrollTop));
  } catch {
    // sessionStorage can be unavailable (private mode, quota); skip silently.
  }
}

export function readScrollPosition(path: string): number | null {
  try {
    const raw = window.sessionStorage.getItem(key(path));
    if (raw === null) {
      return null;
    }
    const value = Number(raw);
    return Number.isFinite(value) && value > 0 ? value : null;
  } catch {
    return null;
  }
}
