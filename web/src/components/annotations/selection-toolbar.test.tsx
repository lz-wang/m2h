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
import {
  SelectionToolbar,
  type SelectionToolbarProps,
} from "./selection-toolbar";

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
  let rangeRects: DOMRect[];
  const originalGetClientRects = Object.getOwnPropertyDescriptor(
    Range.prototype,
    "getClientRects",
  );

  beforeEach(() => {
    body = mountBody("<p>建议使用 SQLite 作为持久化存储。</p>");
    index = new TextIndex(body);
    rangeRects = [new DOMRect(100, 120, 120, 24)];
    Object.defineProperty(Range.prototype, "getClientRects", {
      configurable: true,
      value: () => rangeRects,
    });
  });

  afterEach(() => {
    if (originalGetClientRects) {
      Object.defineProperty(
        Range.prototype,
        "getClientRects",
        originalGetClientRects,
      );
    } else {
      Reflect.deleteProperty(Range.prototype, "getClientRects");
    }
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

  it("follows visible selection fragments across scroll and keeps the native selection", async () => {
    const header = document.createElement("header");
    vi.spyOn(header, "getBoundingClientRect").mockReturnValue(
      new DOMRect(0, 0, 600, 48),
    );
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        readerToolbarRef={{ current: header }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
      />,
    );

    select(2, 11);
    const toolbar = await screen.findByRole("toolbar", {
      name: "标注选中文本",
    });
    expect(toolbar.style.top).toBe("112px");
    rangeRects = [new DOMRect(100, 22, 120, 22), new DOMRect(120, 52, 120, 22)];
    fireEvent.scroll(document);
    await waitFor(() => expect(toolbar.style.top).toBe("82px"));
    expect(toolbar.style.left).toBe("180px");

    rangeRects = [new DOMRect(100, 12, 120, 20)];
    fireEvent.scroll(document);
    await waitFor(() => expect(screen.queryByRole("toolbar")).toBeNull());
    expect(document.getSelection()?.toString()).toBe("使用 SQLite");

    rangeRects = [new DOMRect(100, 140, 120, 24)];
    fireEvent.scroll(document);
    expect((await screen.findByRole("toolbar")).style.top).toBe("132px");

    await userEvent.click(screen.getByRole("button", { name: "标注" }));
    const editor =
      screen.getByPlaceholderText<HTMLTextAreaElement>("添加评论…");
    await userEvent.type(editor, "草稿");
    rangeRects = [];
    await act(async () => {
      fireEvent.scroll(document);
      await new Promise((resolve) => setTimeout(resolve, 0));
    });
    expect(editor.value).toBe("草稿");
    expect(screen.getByRole("region", { name: "新建标注" })).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(screen.queryByRole("toolbar")).toBeNull();
    expect(document.getSelection()?.toString()).toBe("使用 SQLite");
  });

  it("offers one annotation action and saves a highlight with an empty comment", async () => {
    const onAnnotate = vi.fn<SelectionToolbarProps["onAnnotate"]>();
    onAnnotate.mockResolvedValue(true);
    const bodyRef = { current: body };
    const indexRef = { current: index };
    render(
      <SelectionToolbar
        bodyRef={bodyRef}
        indexRef={indexRef}
        onAnnotate={onAnnotate}
      />,
    );

    select(2, 11);
    const toolbar = await screen.findByRole("toolbar", {
      name: "标注选中文本",
    });
    expect(toolbar.querySelectorAll("button")).toHaveLength(1);
    await userEvent.click(screen.getByRole("button", { name: "标注" }));

    expect(
      screen
        .getByRole("button", { name: "琥珀色" })
        .getAttribute("aria-pressed"),
    ).toBe("true");
    await userEvent.click(screen.getByRole("button", { name: "绿色" }));
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(onAnnotate).toHaveBeenCalledTimes(1));
    expect(onAnnotate).toHaveBeenCalledWith(
      expect.objectContaining({
        quote: expect.objectContaining({ exact: "使用 SQLite" }),
        position: expect.objectContaining({ start: 2 }),
      }),
      "",
      "green",
      undefined,
    );
    // A saved annotation clears the toolbar.
    expect(screen.queryByRole("toolbar", { name: "标注选中文本" })).toBeNull();
    select(0, 2);
    await userEvent.click(await screen.findByRole("button", { name: "标注" }));
    const amber = await screen.findByRole("button", { name: "琥珀色" });
    expect(amber.getAttribute("aria-pressed")).toBe("true");
  });

  it("opens the comment editor and submits its text", async () => {
    const onAnnotate = vi.fn<SelectionToolbarProps["onAnnotate"]>();
    onAnnotate.mockResolvedValue(true);
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={onAnnotate}
      />,
    );

    select(0, 2);
    await userEvent.click(await screen.findByRole("button", { name: "标注" }));
    const editor = await screen.findByPlaceholderText("添加评论…");
    await userEvent.type(editor, "需要补充备份");
    await userEvent.click(screen.getByRole("button", { name: "红色" }));
    expect(editor.getAttribute("data-annotation-color")).toBe("red");
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() => expect(onAnnotate).toHaveBeenCalledTimes(1));
    expect(onAnnotate.mock.calls[0]?.[1]).toBe("需要补充备份");
    expect(onAnnotate.mock.calls[0]?.[2]).toBe("red");
  });

  it("restores the original selection when comment editing is canceled", async () => {
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={vi.fn()}
      />,
    );
    select(2, 11);
    await userEvent.click(await screen.findByRole("button", { name: "标注" }));
    await userEvent.click(screen.getByRole("button", { name: "取消" }));
    expect(document.getSelection()?.toString()).toBe("使用 SQLite");
    expect(screen.getByRole("button", { name: "标注" })).toBeTruthy();
  });

  it("keeps the draft and its chosen color when saving fails", async () => {
    const onAnnotate = vi.fn<SelectionToolbarProps["onAnnotate"]>();
    onAnnotate.mockResolvedValue(false);
    render(
      <SelectionToolbar
        bodyRef={{ current: body }}
        indexRef={{ current: index }}
        onAnnotate={onAnnotate}
      />,
    );

    select(0, 2);
    await userEvent.click(await screen.findByRole("button", { name: "标注" }));
    const editor =
      await screen.findByPlaceholderText<HTMLTextAreaElement>("添加评论…");
    await userEvent.click(screen.getByRole("button", { name: "紫色" }));
    await userEvent.type(editor, "草稿");
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    expect((await screen.findByRole("alert")).textContent).toContain(
      "保存失败",
    );
    expect(editor.value).toBe("草稿");
    expect(editor.getAttribute("data-annotation-color")).toBe("purple");
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
