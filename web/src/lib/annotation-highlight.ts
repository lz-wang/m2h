// Highlight rendering through the CSS Custom Highlight API. The registry
// is global (CSS.highlights), so the layer owns cleanup: unmounting the
// document or switching bodies always releases the registered highlight
// groups.
//
// Browsers without the Highlight API keep every other annotation feature —
// the panel lists and locates annotations — and simply see no in-body
// coloring.

import type { TextIndex, TextPosition } from "./annotation-selector";

export const ANNOTATION_HIGHLIGHT_NAME = "m2h-annotations";

interface HighlightConstructor {
  new (...ranges: Range[]): unknown;
}

interface HighlightRegistryLike {
  set(name: string, highlight: unknown): unknown;
  delete(name: string): boolean;
}

function highlightRegistry(): HighlightRegistryLike | null {
  const cssLike = globalThis.CSS as
    | (typeof CSS & { highlights?: HighlightRegistryLike })
    | undefined;
  if (cssLike === undefined || cssLike.highlights === undefined) {
    return null;
  }
  return cssLike.highlights;
}

function highlightConstructor(): HighlightConstructor | null {
  return (globalThis as { Highlight?: HighlightConstructor }).Highlight ?? null;
}

// supportsHighlights reports whether in-body highlighting is available.
export function supportsHighlights(): boolean {
  return highlightRegistry() !== null && highlightConstructor() !== null;
}

// HighlightLayer maintains the registered highlight group for resolved
// annotations. Unresolvable annotations (orphaned or ambiguous) are not
// registered — the panel carries their status instead.
export class HighlightLayer {
  private index: TextIndex | null = null;
  private registered = false;

  // attach binds the live body the ranges resolve against; detaching with
  // null invalidates the layer until the next attach.
  attach(index: TextIndex | null): void {
    this.index = index;
  }

  // render rebuilds the group from the resolved spans of the current body.
  render(positions: TextPosition[]): void {
    const registry = highlightRegistry();
    const highlight = highlightConstructor();
    if (registry === null || highlight === null) {
      return;
    }
    const ranges =
      this.index === null
        ? []
        : positions
            .map((position) => this.index?.rangeFromSpan(position) ?? null)
            .filter((range): range is Range => range !== null);
    if (ranges.length === 0) {
      registry.delete(ANNOTATION_HIGHLIGHT_NAME);
      this.registered = false;
      return;
    }
    registry.set(ANNOTATION_HIGHLIGHT_NAME, new highlight(...ranges));
    this.registered = true;
  }

  // release drops the registered group; call on unmount and body swaps.
  release(): void {
    const registry = highlightRegistry();
    if (registry !== null) {
      registry.delete(ANNOTATION_HIGHLIGHT_NAME);
    }
    this.registered = false;
  }

  get isRegistered(): boolean {
    return this.registered;
  }
}
