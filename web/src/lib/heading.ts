// Heading DOM helpers shared by the scroll spy and heading navigation.
//
// Lookups are scoped to the rendered Markdown body (`.markdown-body`) found
// inside the supplied container, falling back to the container itself when no
// article is present. Scoping matters: a Markdown heading id must never resolve
// to an unrelated element on the WebUI shell (toolbar, sidebar, frontmatter),
// so the global `document.getElementById` is deliberately avoided.
export function findHeadingElement(
  container: HTMLElement,
  id: string,
): HTMLElement | null {
  const scope =
    container.querySelector<HTMLElement>(".markdown-body") ?? container;
  return scope.querySelector<HTMLElement>(`[id="${CSS.escape(id)}"]`);
}

// The reader body scrolls inside a Base UI ScrollArea viewport rather than the
// window. This locates that viewport inside the reader container so the scroll
// spy, the scroll-position saver and the deep-link restore all agree on it.
export function findReaderViewport(container: HTMLElement): HTMLElement | null {
  return container.querySelector<HTMLElement>(
    '[data-slot="scroll-area-viewport"]',
  );
}
