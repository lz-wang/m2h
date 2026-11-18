import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { AnnotationItem } from "@/api";
import { AnnotationPanel } from "./annotation-panel";

function item(overrides: Partial<AnnotationItem> = {}): AnnotationItem {
  return {
    id: "a1",
    authorId: "local:alice",
    authorName: "alice",
    comment: "需要补充备份方案",
    quote: "使用 SQLite",
    selector: {
      quote: { exact: "使用 SQLite", prefix: "", suffix: "" },
      position: { start: 0, end: 9 },
      headingId: "存储",
      version: 1,
    },
    documentHash: "sha256:abc",
    mine: true,
    createdAt: 1,
    updatedAt: 1,
    ...overrides,
  };
}

const document_ = {
  title: "设计文档",
  path: "design.md",
  ref: { rootId: "root-1", path: "design.md" },
  hash: "sha256:abc",
};

function renderPanel(
  overrides: Partial<Parameters<typeof AnnotationPanel>[0]> = {},
) {
  const onLocate = vi.fn();
  const onDelete = vi.fn();
  const onOpenChange = vi.fn();
  const view = render(
    <AnnotationPanel
      open
      onOpenChange={onOpenChange}
      items={[item()]}
      statuses={{}}
      loading={false}
      error={null}
      selectedId={null}
      document={document_}
      onLocate={onLocate}
      onDelete={onDelete}
      {...overrides}
    />,
  );
  return { onLocate, onDelete, onOpenChange, view };
}

describe("AnnotationPanel", () => {
  afterEach(() => {
    document.body.replaceChildren();
  });

  it("lists quotes, comments, authors and anchor states", () => {
    renderPanel({
      statuses: { a1: "orphaned" },
      items: [item()],
    });
    expect(screen.getByText("使用 SQLite")).toBeTruthy();
    expect(screen.getByText("需要补充备份方案")).toBeTruthy();
    expect(screen.getByText("alice")).toBeTruthy();
    expect(screen.getByText("未定位")).toBeTruthy();
  });

  it("locates an annotation when its row action fires", async () => {
    const { onLocate } = renderPanel();
    await userEvent.click(screen.getByRole("button", { name: "定位到正文" }));
    expect(onLocate).toHaveBeenCalledWith("a1");
  });

  it("deletes own annotations only after confirmation", async () => {
    const { onDelete } = renderPanel();
    await userEvent.click(screen.getByRole("button", { name: "删除此标注" }));
    expect(onDelete).not.toHaveBeenCalled();
    await userEvent.click(screen.getByRole("button", { name: "确认删除" }));
    expect(onDelete).toHaveBeenCalledWith("a1");
  });

  it("offers no delete control for other users' annotations", () => {
    renderPanel({ items: [item({ mine: false })] });
    expect(screen.queryByRole("button", { name: "删除此标注" })).toBeNull();
  });

  it("edits the author's comment from the list", async () => {
    const onEditComment = vi.fn().mockResolvedValue(true);
    renderPanel({ onEditComment });
    await userEvent.click(screen.getByRole("button", { name: "编辑此评论" }));
    const editor = screen.getByPlaceholderText("添加评论…");
    await userEvent.clear(editor);
    await userEvent.type(editor, "新评论");
    await userEvent.click(screen.getByRole("button", { name: "保存" }));
    await waitFor(() =>
      expect(onEditComment).toHaveBeenCalledWith("a1", "新评论"),
    );
    await waitFor(() =>
      expect(screen.queryByPlaceholderText("添加评论…")).toBeNull(),
    );
  });

  it("exports the whole set as Markdown and JSON via the clipboard", async () => {
    const onCopy = vi.fn().mockResolvedValue(true);
    renderPanel({ onCopy });
    await userEvent.click(
      screen.getByRole("button", { name: "复制全部标注（Markdown）" }),
    );
    await waitFor(() => expect(onCopy).toHaveBeenCalledWith("markdown"));
    await userEvent.click(
      screen.getByRole("button", { name: "复制全部标注（JSON）" }),
    );
    await waitFor(() => expect(onCopy).toHaveBeenCalledWith("json"));
    await userEvent.click(
      screen.getByRole("button", { name: "复制此标注（JSON）" }),
    );
    await waitFor(() => expect(onCopy).toHaveBeenCalledWith("json", ["a1"]));
    expect(screen.queryByText(/已复制/)).toBeNull();
  });

  it("shows an error when copying fails instead of a success check", async () => {
    const onCopy = vi.fn().mockResolvedValue(false);
    renderPanel({ onCopy });
    const button = screen.getByRole("button", {
      name: "复制全部标注（Markdown）",
    });
    await userEvent.click(button);
    await screen.findByRole("alert");
    expect(button.querySelector(".lucide-check")).toBeNull();
    expect(button.getAttribute("title")).toBe("复制失败，请重试");
  });

  it("shows the empty state without a header close button", () => {
    renderPanel({ items: [], document: null });
    expect(screen.getByText(/还没有标注/)).toBeTruthy();
    expect(screen.queryByRole("button", { name: "关闭标注面板" })).toBeNull();
  });

  it("renders nothing when closed on desktop", () => {
    renderPanel({ open: false });
    expect(screen.queryByRole("region", { name: "标注面板" })).toBeNull();
  });

  it("offers re-anchoring for the author's unresolved annotation", async () => {
    const onReanchor = vi.fn();
    renderPanel({
      statuses: { a1: "orphaned" },
      onReanchor,
    });
    await userEvent.click(screen.getByRole("button", { name: "重新关联" }));
    expect(onReanchor).toHaveBeenCalledWith("a1");
  });

  it("offers no re-anchoring for resolved annotations or others' items", () => {
    renderPanel({
      statuses: { a1: "resolved" },
      onReanchor: vi.fn(),
    });
    expect(screen.queryByRole("button", { name: "重新关联" })).toBeNull();

    renderPanel({
      items: [item({ mine: false })],
      statuses: { a1: "orphaned" },
      onReanchor: vi.fn(),
    });
    expect(screen.queryByRole("button", { name: "重新关联" })).toBeNull();
  });
});
