// Tab selection belongs to native DOM controls. Helpers share that state with
// heading/search navigation; they never create a parallel React store.
export const TAB_ACTIVATED_EVENT = "m2h-tab-activated";

export function findTabPanel(
  root: HTMLElement,
  tabId: string,
): HTMLElement | null {
  return root.querySelector<HTMLElement>(
    `.m2h-tabbed-panel[data-m2h-tab-id="${CSS.escape(tabId)}"]`,
  );
}

function panelRadio(panel: HTMLElement): HTMLInputElement | null {
  const set = panel.closest<HTMLElement>(".m2h-tabbed-set");
  const id = panel.dataset.m2hTabId;
  if (set === null || id === undefined) return null;
  return set.querySelector<HTMLInputElement>(
    `:scope > input.m2h-tabbed-radio[id="${CSS.escape(id)}"]`,
  );
}

export function activateTabForElement(element: HTMLElement): boolean {
  const panel = element.closest<HTMLElement>(".m2h-tabbed-panel");
  if (panel === null) return false;
  const radio = panelRadio(panel);
  if (radio === null || radio.checked) return false;
  radio.checked = true;
  // Rich-content enhancement listens to both native changes and navigation.
  // Keep programmatic activation distinct from a user's manual change so it
  // cannot clear the fragment that navigation is restoring.
  radio.dispatchEvent(new Event(TAB_ACTIVATED_EVENT, { bubbles: true }));
  return true;
}

export function activateTabById(root: HTMLElement, tabId: string): boolean {
  const panel = findTabPanel(root, tabId);
  return panel !== null && activateTabForElement(panel);
}

export function isElementInInactiveTab(element: HTMLElement): boolean {
  const panel = element.closest<HTMLElement>(".m2h-tabbed-panel");
  if (panel === null) return false;
  const radio = panelRadio(panel);
  return radio !== null && !radio.checked;
}

export function isTabRadio(
  target: EventTarget | null,
): target is HTMLInputElement {
  return (
    target instanceof HTMLInputElement &&
    target.matches(".m2h-tabbed-set > .m2h-tabbed-radio")
  );
}
