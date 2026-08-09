import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { App } from "./App";

describe("App", () => {
  it("renders the directory preview placeholder", () => {
    expect(renderToStaticMarkup(<App />)).toContain("目录预览将在阶段 6 提供");
  });
});
