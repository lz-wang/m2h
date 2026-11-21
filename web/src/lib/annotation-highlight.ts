// Highlight rendering through the CSS Custom Highlight API. The registry
// is global (CSS.highlights), so the layer owns cleanup: unmounting the
// document or switching bodies always releases the registered highlight
// groups.
//
// Browsers without the Highlight API keep every other annotation feature —
// the panel lists and locates annotations — and simply see no in-body
// coloring.

import { ANNOTATION_COLORS, type AnnotationColor } from "@/api";
import type { TextIndex, TextPosition } from "./annotation-selector";

export const ANNOTATION_HIGHLIGHT_NAME = "m2h-annotations";
export const DRAFT_ANNOTATION_HIGHLIGHT_NAME = "m2h-annotation-draft";

function colorHighlightName(name: string, color: AnnotationColor): string {
  return color === "blue" ? name : `${name}-${color}`;
}

export function draftHighlightName(color: AnnotationColor): string {
  return colorHighlightName(DRAFT_ANNOTATION_HIGHLIGHT_NAME, color);
}

interface HighlightConstructor {
  new (...ranges: Range[]): { priority: number };
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

  constructor(
    private readonly name = ANNOTATION_HIGHLIGHT_NAME,
    private readonly priority = 0,
  ) {}

  // attach binds the live body the ranges resolve against; detaching with
  // null invalidates the layer until the next attach.
  attach(index: TextIndex | null): void {
    this.index = index;
  }

  // render rebuilds the group from the resolved spans of the current body.
  render(positions: TextPosition[]): void {
    const index = this.index;
    this.renderRanges(
      index === null
        ? []
        : positions
            .map((position) => index.rangeFromSpan(position))
            .filter((range): range is Range => range !== null),
    );
  }

  renderRanges(ranges: Range[]): void {
    const registry = highlightRegistry();
    const highlight = highlightConstructor();
    if (registry === null || highlight === null) {
      return;
    }
    if (ranges.length === 0) {
      registry.delete(this.name);
      this.registered = false;
      return;
    }
    const group = new highlight(...ranges);
    group.priority = this.priority;
    registry.set(this.name, group);
    this.registered = true;
  }

  // release drops the registered group; call on unmount and body swaps.
  release(): void {
    const registry = highlightRegistry();
    if (registry !== null) {
      registry.delete(this.name);
    }
    this.registered = false;
  }

  get isRegistered(): boolean {
    return this.registered;
  }
}

export interface AnnotationHighlight {
  id: string;
  color: AnnotationColor;
  position: TextPosition;
}

// Split the painted spans at overlap boundaries. The newest annotation
// owns each shared span so translucent backgrounds never mix. Hit testing
// still uses the original, overlapping positions.
function visibleSpans(
  annotations: AnnotationHighlight[],
): AnnotationHighlight[] {
  const edges = annotations
    .flatMap(({ position }, index) => [
      { offset: position.start, index, start: true },
      { offset: position.end, index, start: false },
    ])
    .sort((a, b) => a.offset - b.offset);
  const active = new Set<number>();
  const visible: AnnotationHighlight[] = [];
  let winner = -1;
  let previous = 0;
  for (const edge of edges) {
    const annotation = annotations[winner];
    if (annotation && previous < edge.offset) {
      const last = visible.at(-1);
      if (last?.id === annotation.id && last.position.end === previous) {
        last.position.end = edge.offset;
      } else {
        visible.push({
          ...annotation,
          position: { start: previous, end: edge.offset },
        });
      }
    }
    if (edge.start) {
      active.add(edge.index);
      winner = Math.max(winner, edge.index);
    } else {
      active.delete(edge.index);
      if (winner === edge.index) {
        winner = -1;
        for (const candidate of active) winner = Math.max(winner, candidate);
      }
    }
    previous = edge.offset;
  }
  return visible;
}

export class AnnotationHighlightLayer {
  private readonly groups = ANNOTATION_COLORS.flatMap((color) => {
    const name = colorHighlightName(ANNOTATION_HIGHLIGHT_NAME, color);
    return [
      { color, emphasized: false, layer: new HighlightLayer(name) },
      { color, emphasized: true, layer: new HighlightLayer(`${name}-active`) },
    ];
  });
  private spans: Array<{ id: string; color: AnnotationColor; range: Range }> =
    [];
  private selectedId: string | null = null;
  private hoveredId: string | null = null;

  render(index: TextIndex, annotations: AnnotationHighlight[]): void {
    if (!supportsHighlights()) return;
    this.spans = visibleSpans(annotations).flatMap(
      ({ id, color, position }) => {
        const range = index.rangeFromSpan(position);
        return range === null ? [] : [{ id, color, range }];
      },
    );
    this.paint();
  }

  emphasize(selectedId: string | null, hoveredId: string | null): void {
    this.selectedId = selectedId;
    this.hoveredId = hoveredId;
    if (this.spans.length > 0) this.paint();
  }

  release(): void {
    this.spans = [];
    for (const { layer } of this.groups) layer.release();
  }

  private paint(): void {
    for (const { color, emphasized, layer } of this.groups) {
      layer.renderRanges(
        this.spans
          .filter(
            (span) =>
              span.color === color &&
              (span.id === this.selectedId || span.id === this.hoveredId) ===
                emphasized,
          )
          .map((span) => span.range),
      );
    }
  }
}
