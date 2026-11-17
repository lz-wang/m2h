// Re-anchoring: relocating a stored selector on the current body after the
// document changed.
//
// Priority (deliberately exact, never fuzzy): verify the original
// position, search the exact quote, disambiguate with prefix/suffix, then
// with the heading region. A quote the editor paraphrased is NOT the same
// text — meaning-similar sentences must not silently hijack an annotation
// — so anything short of a confident match stays unresolved.

import {
  type AnnotationSelector,
  nearestHeadingId,
  type TextIndex,
  type TextPosition,
} from "./annotation-selector";

export type AnchorStatus = "resolved" | "ambiguous" | "orphaned";

export interface AnchorResult {
  status: AnchorStatus;
  // The live position when resolved; null otherwise.
  position: TextPosition | null;
  // Selector as it would be stored after a human-confirmed re-anchor: the
  // new position plus refreshed context (kept only when resolved).
  refreshed: AnnotationSelector | null;
}

// reanchor locates one stored selector on the index of the current body.
export function reanchor(
  index: TextIndex,
  selector: AnnotationSelector,
): AnchorResult {
  const exact = selector.quote.exact;

  // Step 1: the stored position still holds the quoted text — resolved
  // without any search.
  if (positionValid(index, selector.position, exact)) {
    return {
      status: "resolved",
      position: selector.position,
      refreshed: refreshSelector(index, selector, selector.position),
    };
  }

  // Step 2: every candidate occurrence of the exact quote.
  const candidates = index.findAll(exact);
  if (candidates.length === 0) {
    return { status: "orphaned", position: null, refreshed: null };
  }

  // Step 3: prefix/suffix context narrows the candidates.
  const contextual = candidates.filter((start) =>
    contextMatches(index, selector, start, exact.length),
  );
  const narrowed = contextual.length > 0 ? contextual : candidates;

  // Step 4: the heading region settles remaining ties. A candidate under
  // the stored heading (or, without a stored heading, the first) wins only
  // when unique.
  let winner: number | null = null;
  let unique = false;
  if (narrowed.length === 1) {
    winner = narrowed[0];
    unique = true;
  } else if (selector.headingId !== undefined) {
    const inHeading = narrowed.filter(
      (start) => headingAt(index, start) === selector.headingId,
    );
    if (inHeading.length === 1) {
      winner = inHeading[0];
      unique = true;
    }
  }

  if (winner === null || !unique) {
    return {
      status: narrowed.length > 0 && winner === null ? "ambiguous" : "orphaned",
      position: null,
      refreshed: null,
    };
  }

  const position: TextPosition = {
    start: winner,
    end: winner + runeLength(exact),
  };
  return {
    status: "resolved",
    position,
    refreshed: refreshSelector(index, selector, position),
  };
}

function positionValid(
  index: TextIndex,
  position: TextPosition,
  exact: string,
): boolean {
  if (position.end > index.length || position.end <= position.start) {
    return false;
  }
  return index.slice(position.start, position.end) === exact;
}

function contextMatches(
  index: TextIndex,
  selector: AnnotationSelector,
  start: number,
  exactRunes: number,
): boolean {
  const { prefix, suffix } = selector.quote;
  if (prefix !== "") {
    const before = index.slice(Math.max(0, start - runeLength(prefix)), start);
    if (before !== prefix) {
      return false;
    }
  }
  if (suffix !== "") {
    const after = index.slice(
      start + exactRunes,
      start + exactRunes + runeLength(suffix),
    );
    if (after !== suffix) {
      return false;
    }
  }
  return true;
}

// headingAt finds the heading governing a canonical-text position: resolve
// the position onto the DOM, then reuse the selector module's traversal.
function headingAt(index: TextIndex, start: number): string | undefined {
  const boundary = index.rangeFromSpan({ start, end: start + 1 });
  if (boundary === null) {
    return undefined;
  }
  const element =
    boundary.startContainer instanceof Element
      ? boundary.startContainer
      : boundary.startContainer.parentElement;
  if (element === null) {
    return undefined;
  }
  return nearestHeadingId(index, element);
}

function refreshSelector(
  index: TextIndex,
  selector: AnnotationSelector,
  position: TextPosition,
): AnnotationSelector {
  return {
    ...selector,
    quote: {
      exact: selector.quote.exact,
      ...index.context(position),
    },
    position,
  };
}

function runeLength(value: string): number {
  let count = 0;
  for (let index = 0; index < value.length; ) {
    const codePoint = value.codePointAt(index);
    if (codePoint === undefined) {
      break;
    }
    index += codePoint > 0xffff ? 2 : 1;
    count += 1;
  }
  return count;
}
