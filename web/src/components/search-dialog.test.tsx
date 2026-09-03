import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { SearchResult } from "@/api";
import type { UseSearchState } from "@/use-search";
import { SearchDialog } from "./search-dialog";

function createState(overrides: Partial<UseSearchState> = {}): UseSearchState {
  return {
    query: "",
    setQuery: vi.fn(),
    results: [],
    phase: "idle",
    error: null,
    reset: vi.fn(),
    ...overrides,
  };
}

const sampleResults: SearchResult[] = [
  {
    path: "docs/markdown.md",
    title: "Markdown Rendering",
    snippet: "…解析通过 Goldmark AST 完成…",
    heading: { id: "parser", text: "Parser" },
  },
  {
    path: "README.md",
    title: "Readme",
  },
];

interface RenderOptions {
  state?: UseSearchState;
  open?: boolean;
}

// The query input is type="search", so its ARIA role is searchbox.
function searchInput(): HTMLElement {
  return screen.getByRole("searchbox", { name: "全文搜索" });
}

function renderDialog({
  state = createState(),
  open = true,
}: RenderOptions = {}) {
  const onOpenChange = vi.fn();
  const onSelect = vi.fn();
  render(
    <SearchDialog
      open={open}
      onOpenChange={onOpenChange}
      search={state}
      onSelect={onSelect}
    />,
  );
  return { state, onOpenChange, onSelect };
}

describe("SearchDialog", () => {
  it("prompts for a keyword while the query is empty", () => {
    renderDialog();
    expect(screen.getByRole("dialog", { name: "全文搜索" })).toBeTruthy();
    expect(screen.getByText("输入关键词搜索当前文档集")).toBeTruthy();
  });

  it("asks for more characters below the auto-search threshold", () => {
    renderDialog({ state: createState({ query: "图" }) });
    expect(screen.getByText("请输入至少 2 个字符")).toBeTruthy();
  });

  it("shows a busy hint while searching", () => {
    renderDialog({
      state: createState({ query: "goldmark", phase: "searching" }),
    });
    expect(screen.getByText("正在搜索…").getAttribute("aria-busy")).toBe(
      "true",
    );
  });

  it("reports an empty result set", () => {
    renderDialog({
      state: createState({ query: "goldmark", phase: "ready" }),
    });
    expect(screen.getByText("没有找到匹配文档")).toBeTruthy();
  });

  it("reports a failed search", () => {
    renderDialog({
      state: createState({ query: "goldmark", phase: "error", error: "x" }),
    });
    expect(screen.getByRole("alert").textContent).toContain("搜索失败，请重试");
  });

  it("renders results with title, section, path and plain-text snippet", () => {
    renderDialog({
      state: createState({
        query: "goldmark",
        phase: "ready",
        results: sampleResults,
      }),
    });
    const options = screen.getAllByRole("button", { name: /，/ });
    expect(options).toHaveLength(2);
    expect(options[0]?.getAttribute("aria-label")).toBe(
      "Markdown Rendering，docs/markdown.md",
    );
    expect(options[0]?.textContent).toContain("Parser");
    expect(options[0]?.textContent).toContain("docs/markdown.md");
    // Snippets are React text, never HTML — markup in a snippet stays visible
    // text instead of becoming DOM.
    expect(options[0]?.textContent).toContain("…解析通过 Goldmark AST 完成…");
    expect(options[1]?.getAttribute("aria-label")).toBe("Readme，README.md");
    expect(
      options[1]?.querySelector(".search-dialog-result-snippet"),
    ).toBeNull();
  });

  it("types into the search state", async () => {
    const user = userEvent.setup();
    const { state } = renderDialog();
    await user.type(searchInput(), "g");
    expect(state.setQuery).toHaveBeenCalledWith("g");
  });

  it("moves the active option with arrow keys and selects with Enter", () => {
    const { onSelect, onOpenChange } = renderDialog({
      state: createState({
        query: "goldmark",
        phase: "ready",
        results: sampleResults,
      }),
    });
    const input = searchInput();
    const options = screen.getAllByRole("button", { name: /，/ });
    expect(options[0]?.getAttribute("aria-current")).toBe("true");

    fireEvent.keyDown(input, { key: "ArrowDown" });
    expect(options[1]?.getAttribute("aria-current")).toBe("true");
    expect(options[0]?.getAttribute("aria-current")).toBeNull();

    fireEvent.keyDown(input, { key: "ArrowUp" });
    fireEvent.keyDown(input, { key: "ArrowUp" });
    // ArrowUp clamps at the first option.
    expect(options[0]?.getAttribute("aria-current")).toBe("true");

    fireEvent.keyDown(input, { key: "Enter" });
    expect(onSelect).toHaveBeenCalledWith(sampleResults[0]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("selects a result by pointer without losing input focus semantics", async () => {
    const user = userEvent.setup();
    const { onSelect, onOpenChange } = renderDialog({
      state: createState({
        query: "goldmark",
        phase: "ready",
        results: sampleResults,
      }),
    });
    await user.click(
      screen.getAllByRole("button", { name: /，/ })[1] as HTMLElement,
    );
    expect(onSelect).toHaveBeenCalledWith(sampleResults[1]);
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("resets the search state when it closes", () => {
    const state = createState({ query: "goldmark", phase: "ready" });
    const { rerender } = render(
      <SearchDialog
        open={true}
        onOpenChange={vi.fn()}
        search={state}
        onSelect={vi.fn()}
      />,
    );
    rerender(
      <SearchDialog
        open={false}
        onOpenChange={vi.fn()}
        search={state}
        onSelect={vi.fn()}
      />,
    );
    expect(state.reset).toHaveBeenCalled();
  });
});
