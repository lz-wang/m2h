import type { RefObject } from "react";

export function watchReaderScroll(
  bodyRef: RefObject<HTMLElement | null>,
  onScroll: (body: HTMLElement) => void,
): () => void {
  let frame: number | null = null;
  const handleScroll = ({ target }: Event) => {
    const body = bodyRef.current;
    if (
      body === null ||
      frame !== null ||
      (target !== window &&
        target !== document &&
        !(
          target instanceof Node &&
          (target.contains(body) || body.contains(target))
        ))
    ) {
      return;
    }
    frame = requestAnimationFrame(() => {
      frame = null;
      if (bodyRef.current === body) onScroll(body);
    });
  };
  window.addEventListener("scroll", handleScroll, {
    capture: true,
    passive: true,
  });
  return () => {
    window.removeEventListener("scroll", handleScroll, true);
    if (frame !== null) cancelAnimationFrame(frame);
  };
}
