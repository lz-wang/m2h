import { readFileSync } from "node:fs";
import { createContext, runInContext } from "node:vm";
import { waitFor } from "@testing-library/react";
import { expect, it } from "vitest";

it("renders parser-owned TeX with the real bundled KaTeX and export runtime", async () => {
  document.documentElement.classList.add("m2h-mode-light");
  document.body.innerHTML = String.raw`<article class="markdown-body"><h1>Math</h1><div class="m2h-math">$$
dp[r][c]
=
\min(dp[r-1][c],dp[r][c-1])
$$
</div><p><span class="m2h-math">$x + \$ + y$</span> Price $9 and $200.</p></article>`;
  const context = createContext({
    document,
    window,
    console,
    Node,
    NodeFilter,
  });
  runInContext("self = globalThis", context);
  try {
    for (const relative of [
      "../../../internal/assets/rich/katex.min.js",
      "../../../internal/assets/rich/auto-render.min.js",
      "../../../internal/export/runtime.js",
    ]) {
      runInContext(
        readFileSync(new URL(relative, import.meta.url), "utf8"),
        context,
      );
    }
    document.dispatchEvent(new Event("DOMContentLoaded"));
    await waitFor(() =>
      expect(document.querySelectorAll(".katex")).toHaveLength(2),
    );
    expect(document.querySelector(".katex-error")).toBeNull();
    expect(document.querySelectorAll(".katex-display")).toHaveLength(1);
    expect(document.querySelectorAll("h1")).toHaveLength(1);
    const tex = Array.from(
      document.querySelectorAll('annotation[encoding="application/x-tex"]'),
      (node) => node.textContent,
    );
    expect(tex).toEqual([
      "\ndp[r][c]\n=\n\\min(dp[r-1][c],dp[r][c-1])\n",
      String.raw`x + \$ + y`,
    ]);
    expect(document.querySelectorAll(".m2h-literal-dollar")).toHaveLength(2);
  } finally {
    document.body.innerHTML = "";
    document.documentElement.classList.remove("m2h-mode-light");
  }
});
