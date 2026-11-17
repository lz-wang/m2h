import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { TextIndex } from "@/lib/annotation-selector";
import { SelectionToolbar } from "./selection-toolbar";

function mountBody(html: string): HTMLElement {
  document.body.replaceChildren();
  const root = document.createElement("article");
  root.innerHTML = html;
  document.body.append(root);
  return root;
}

describe("SelectionToolbar", () => {
  let body: HTMLElement;
  let index: TextIndex;

  beforeEach(() => {
    body = mountBody("<p>建议使用 SQLite 作为持久化存储。</p>");
    index = new TextIndex(body);
  });

  afterEach(() => {
    document.body.replaceChildren();
  });

  function select(start: number, end: number) {
    const text = body.querySelector("p")?.firstChild as Text;
    const selection = document.getSelection();
    selection?.removeAllRanges();
    const range = document.createRange();
    range.setStart(text, start);
    range.setEnd(text, end);
    selection?.addRange(range);
    document.dispatchEvent(new Event("selectionchange"));
  }

  it("shows the actions for a valid content selection and saves highlights", async () => {
    const onAnnotate =
      vi.fn<(selector: unknown, comment: string) => Promise<boolean>>();
    onAnnotate.mockResolvedValue(true);
    const bodyRef = { current: body };
    const indexRef = { current: index };
    render(
      <SelectionToolbar
        bodyRef={bodyRef}
        indexRef={indexRef}
        onAnnotate={onAnnotate as never}
      />,
    );

    select(2, 11);
    await waitFor(() =>
      expect(
        screen.getByRole("toolbar", { name: "标注选中文本" }),
      ).toBeTruthy(),
    );

    await userEvent.click(screen.getByRole("button", { name: "高亮" }));
    await waitFor(() => expect(onAnnotate).toHaveBeenCalledTimes(1));
    const [selector, comment] = onAnnotate.mock.calls[0] as [
      { quote: { exact: string }; position: { start: number } },
      string,
    ];
    expect(selector.quote.exact).toBe("使用 SQLite");
    expect(selector.position.start).toBe(2);
    expect(comment).toBe("");
    // A saved annotation clears the toolbar.
    expect(screen.queryByRole("toolbar", { name: "标注选中文本" })).toBeNull();
  });

  it("opens the comment editor and submits its text", async () => {
    const onAnnotate =
      vi.fn<(selector: unknown, comment: string) => Promise<boolean>>();
    onAnnotate.mockResolvedValue(true);
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={onAnnotate as never}
      />,
    );

    select(0, 2);
    await waitFor(() => expect(screen.getByRole("toolbar")).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "评论" }));
    const editor = await screen.findByPlaceholderText("添加评论…");
    await userEvent.type(editor, "需要补充备份");
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(onAnnotate).toHaveBeenCalledTimes(1));
    expect(onAnnotate.mock.calls[0]?.[1]).toBe("需要补充备份");
  });

  it("keeps the toolbar with a failure notice when saving fails", async () => {
    const onAnnotate =
      vi.fn<(selector: unknown, comment: string) => Promise<boolean>>();
    onAnnotate.mockResolvedValue(false);
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={onAnnotate as never}
      />,
    );

    select(0, 2);
    await waitFor(() => expect(screen.getByRole("toolbar")).toBeTruthy());
    await userEvent.click(screen.getByRole("button", { name: "高亮" }));
    expect((await screen.findByRole("alert")).textContent).toContain(
      "保存失败",
    );
    expect(screen.getByRole("toolbar")).toBeTruthy();
  });

  it("never appears for collapsed or decoration-only selections", () => {
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
      />,
    );
    select(3, 3);
    expect(screen.queryByRole("toolbar")).toBeNull();
  });

  it("hides when disabled", async () => {
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
        disabled
      />,
    );
    select(0, 3);
    // selectionchange is ignored while disabled.
    act(() => {
      document.dispatchEvent(new Event("selectionchange"));
    });
    expect(screen.queryByRole("toolbar")).toBeNull();
  });

  it("dismisses on Escape", async () => {
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
      />,
    );
    select(0, 3);
    await waitFor(() => expect(screen.getByRole("toolbar")).toBeTruthy());
    fireEvent.keyDown(window, { key: "Escape" });
    expect(screen.queryByRole("toolbar")).toBeNull();
  });

  it("re-anchor mode saves the new selection as the confirmed anchor", async () => {
    const onAnnotate = vi.fn();
    const onReanchor = vi.fn<(selector: unknown) => Promise<boolean>>();
    onReanchor.mockResolvedValue(true);
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={onAnnotate as never}
        reanchorPending={{ quote: "旧版本的引用" }}
        onReanchor={onReanchor as never}
      />,
    );

    select(2, 11);
    await waitFor(() =>
      expect(
        screen.getByRole("toolbar", { name: "重新关联标注" }),
      ).toBeTruthy(),
    );
    // The annotation entry stays visible while re-anchoring.
    expect(screen.getByText(/旧版本的引用/)).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "关联到此选区" }));
    await waitFor(() => expect(onReanchor).toHaveBeenCalledTimes(1));
    expect(onAnnotate).not.toHaveBeenCalled();
    const [selector] = onReanchor.mock.calls[0] as [
      { quote: { exact: string } },
    ];
    expect(selector.quote.exact).toBe("使用 SQLite");
    // A confirmed re-anchor clears the toolbar and the selection.
    expect(screen.queryByRole("toolbar")).toBeNull();
  });

  it("re-anchor mode cancels on Escape", async () => {
    const onReanchorCancel = vi.fn();
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
        reanchorPending={{ quote: "旧版本的引用" }}
        onReanchorCancel={onReanchorCancel}
      />,
    );
    select(0, 3);
    await waitFor(() => expect(screen.getByRole("toolbar")).toBeTruthy());
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onReanchorCancel).toHaveBeenCalledTimes(1);
  });

  it("re-anchor mode cancels on Escape with NO selection yet", async () => {
    // The banner promises "press Esc to cancel" — most of the flow is
    // spent with nothing selected, so Escape must work in that state too.
    const onReanchorCancel = vi.fn();
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
        reanchorPending={{ quote: "旧版本的引用" }}
        onReanchorCancel={onReanchorCancel}
      />,
    );
    expect(screen.queryByRole("toolbar")).toBeNull();
    fireEvent.keyDown(window, { key: "Escape" });
    expect(onReanchorCancel).toHaveBeenCalledTimes(1);
  });
});
