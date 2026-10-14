import { afterEach, describe, expect, it, vi } from "vitest";
import {
  activateTabById,
  activateTabForElement,
  findTabPanel,
  isElementInInactiveTab,
  TAB_ACTIVATED_EVENT,
} from "./tabbed";

function tabFixture(): HTMLElement {
  const root = document.createElement("div");
  root.innerHTML = `<div class="m2h-tabbed-set">
  <input class="m2h-tabbed-radio" type="radio" name="a" id="first" checked>
  <input class="m2h-tabbed-radio" type="radio" name="a" id="second">
  <div class="m2h-tabbed-content">
    <section class="m2h-tabbed-panel" data-m2h-tab-id="first"><h2 id="one">One</h2></section>
    <section class="m2h-tabbed-panel" data-m2h-tab-id="second"><h2 id="two">Two</h2></section>
  </div></div><h2 id="outside">Outside</h2>`;
  document.body.append(root);
  return root;
}

describe("native tab DOM helpers", () => {
  afterEach(() => {
    document.body.replaceChildren();
  });
  it("activates only the target group and announces navigation once", () => {
    const root = tabFixture();
    const second = root.querySelector<HTMLElement>("#two");
    if (second === null) throw new Error("fixture");
    const listener = vi.fn();
    root.addEventListener(TAB_ACTIVATED_EVENT, listener);
    expect(isElementInInactiveTab(second)).toBe(true);
    expect(activateTabForElement(second)).toBe(true);
    expect(isElementInInactiveTab(second)).toBe(false);
    expect(root.querySelector<HTMLInputElement>("#first")?.checked).toBe(false);
    expect(activateTabById(root, "second")).toBe(false);
    expect(listener).toHaveBeenCalledTimes(1);
    expect(activateTabById(root, "missing")).toBe(false);
  });
  it("leaves ordinary and stacked content alone, scopes panel lookups", () => {
    const root = tabFixture();
    const other = tabFixture();
    const outside = root.querySelector<HTMLElement>("#outside");
    if (outside === null) throw new Error("fixture");
    expect(activateTabForElement(outside)).toBe(false);
    expect(isElementInInactiveTab(outside)).toBe(false);
    expect(findTabPanel(root, "second")).not.toBe(
      findTabPanel(other, "second"),
    );
    root.querySelectorAll("input").forEach((input) => {
      input.remove();
    });
    const panel = findTabPanel(root, "second");
    if (panel === null) throw new Error("fixture");
    expect(activateTabForElement(panel)).toBe(false);
    expect(isElementInInactiveTab(panel)).toBe(false);
  });
});
