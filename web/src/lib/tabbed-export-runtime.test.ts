import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { createContext, runInContext } from "node:vm";
import { waitFor } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";

const runtime = readFileSync(
  resolve(process.cwd(), "../internal/export/runtime.js"),
  "utf8",
);

afterEach(() => {
  document.body.replaceChildren();
  window.history.replaceState(null, "", "/");
  vi.restoreAllMocks();
});

it("restores hidden headings on initial hash and hashchange without CDN", () => {
  document.body.innerHTML = `<article class="markdown-body"><div class="m2h-tabbed-set"><input class="m2h-tabbed-radio" type="radio" name="exporttab" id="one" checked><input class="m2h-tabbed-radio" type="radio" name="exporttab" id="two"><section class="m2h-tabbed-panel" data-m2h-tab-id="one"><h2 id="first">First</h2></section><section class="m2h-tabbed-panel" data-m2h-tab-id="two"><h2 id="安装-1">Second</h2></section></div></article>`;
  window.history.replaceState(null, "", "/#%E5%AE%89%E8%A3%85-1");
  const scroll = vi
    .spyOn(Element.prototype, "scrollIntoView")
    .mockImplementation(() => {});
  const listeners: EventListener[] = [];
  const add = vi
    .spyOn(window, "addEventListener")
    .mockImplementation((name, listener) => {
      if (name === "hashchange" && typeof listener === "function")
        listeners.push(listener);
    });
  runInContext(
    runtime,
    createContext({ document, window, NodeFilter, HTMLPreElement }),
  );
  document.dispatchEvent(new Event("DOMContentLoaded"));
  expect(document.querySelector<HTMLInputElement>("#two")?.checked).toBe(true);
  expect(scroll).toHaveBeenCalled();
  window.history.replaceState(null, "", "/#first");
  listeners.forEach((listener) => {
    listener(new Event("hashchange"));
  });
  expect(document.querySelector<HTMLInputElement>("#one")?.checked).toBe(true);
  window.history.replaceState(null, "", "/#%broken");
  listeners.forEach((listener) => {
    listener(new Event("hashchange"));
  });
  expect(document.querySelector<HTMLInputElement>("#one")?.checked).toBe(true);
  add.mockRestore();
});

it("renders hidden Mermaid offscreen once and isolates failed diagrams", async () => {
  document.body.innerHTML = `<article class="markdown-body"><section style="display:none"><pre><code class="language-mermaid">graph TD; A--&gt;B</code></pre><pre><code class="language-mermaid">bad diagram</code></pre></section><pre><code class="language-mermaid">graph TD; C--&gt;D</code></pre></article>`;
  const mermaid = {
    initialize: vi.fn(),
    render: vi
      .fn()
      .mockResolvedValueOnce({ svg: '<svg viewBox="0 0 100 50"></svg>' })
      .mockRejectedValueOnce(new Error("bad diagram"))
      .mockResolvedValueOnce({ svg: "<svg></svg>" }),
  };
  runInContext(
    runtime,
    createContext({ document, window, NodeFilter, HTMLPreElement, mermaid }),
  );
  document.dispatchEvent(new Event("DOMContentLoaded"));
  await waitFor(() =>
    expect(document.querySelectorAll(".mermaid svg")).toHaveLength(2),
  );
  expect(mermaid.render).toHaveBeenCalledTimes(3);
  expect(document.body.textContent).toContain("Mermaid 图表渲染失败");
});
