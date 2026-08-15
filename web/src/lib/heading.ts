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
