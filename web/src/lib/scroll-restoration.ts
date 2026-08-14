// Scroll restoration driven by layout stability instead of a fixed frame
// count.
//
// A resolved rich-content render promise does not mean the browser has
// finished the final layout: async enhancements (Mermaid, KaTeX, Tablesort,
// late-loading images, ScrollArea resize observers) can keep changing the
// body for several more frames. Re-applying a saved offset across a hardcoded
// number of frames is a timing guess that breaks again whenever a new
// enhancement joins the pipeline. This module instead watches the viewport's
// scrollHeight and settles only once it has stopped changing for several
// consecutive frames.

const stableFramesRequired = 3;
const maxFrames = 30;

/**
 * Drive `viewport.scrollTop` to `targetScrollTop` and keep re-applying the
 * offset every frame until the reader layout has stayed identical for
 * `stableFramesRequired` consecutive frames — or `maxFrames` frames have
 * passed, so a page that never settles cannot hold the restore guard
 * forever. Once the final layout is in, the target is written one last time
 * and `onSettled` fires.
 *
 * While the restore is in flight, CSS scroll anchoring is suppressed on the
 * viewport (`overflow-anchor: none`): the browser must not compensate for
 * content changes above the viewport and compete with the explicit
 * `scrollTop`. The previous inline value is restored when the loop settles
 * or is cancelled — scroll anchoring stays active during normal reading,
 * where it is valuable.
 *
 * Returns a cancel function that stops the loop and restores
 * `overflow-anchor` without writing the scroll position or calling
 * `onSettled`: the caller keeps owning the restore-guard lifecycle.
 */
export function restoreScrollTopWhenStable(
  viewport: HTMLElement,
  targetScrollTop: number,
  onSettled: () => void,
): () => void {
  let frameID = 0;
  let frameCount = 0;
  let stableFrames = 0;
  let lastScrollHeight = -1;

  const previousOverflowAnchor = viewport.style.overflowAnchor;

  // The application owns scroll restoration during this interval. Prevent CSS
  // scroll anchoring from competing with the explicit scrollTop.
  viewport.style.overflowAnchor = "none";

  const finish = () => {
    cancelAnimationFrame(frameID);

    // Re-apply the frozen target after the final layout pass.
    viewport.scrollTop = targetScrollTop;
    viewport.style.overflowAnchor = previousOverflowAnchor;

    onSettled();
  };

  const tick = () => {
    frameCount += 1;

    // Re-apply the frozen target after every layout pass.
    viewport.scrollTop = targetScrollTop;

    const scrollHeight = viewport.scrollHeight;

    if (scrollHeight === lastScrollHeight) {
      stableFrames += 1;
    } else {
      stableFrames = 0;
      lastScrollHeight = scrollHeight;
    }

    if (stableFrames >= stableFramesRequired || frameCount >= maxFrames) {
      finish();
      return;
    }

    frameID = requestAnimationFrame(tick);
  };

  frameID = requestAnimationFrame(tick);

  return () => {
    cancelAnimationFrame(frameID);
    viewport.style.overflowAnchor = previousOverflowAnchor;
  };
}
