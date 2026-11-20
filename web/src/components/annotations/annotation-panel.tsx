import { Crosshair, LocateFixed, Pencil, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AnnotationItem } from "@/api";
import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { ExportDocument } from "@/lib/annotation-export";
import { AnnotationAction } from "./annotation-action";
import { AnnotationCommentMeta } from "./annotation-comment-meta";
import {
  AnnotationCopyButton,
  type AnnotationCopyFormat,
} from "./annotation-copy-button";
import { AnnotationEditor } from "./annotation-editor";

export type AnnotationAnchorStatus = "resolved" | "ambiguous" | "orphaned";

export interface AnnotationPanelProps {
  open: boolean;
  onOpenChange(open: boolean): void;
  items: AnnotationItem[];
  statuses: Record<string, AnnotationAnchorStatus>;
  loading: boolean;
  error: string | null;
  selectedId: string | null;
  selectionVersion?: number;
  document: ExportDocument | null;
  // Scrolls the body to the annotation's live position.
  onLocate(id: string): void;
  onDelete(ids: string | string[]): Promise<void>;
  onEditComment?(id: string, comment: string): Promise<boolean>;
  // Starts the human re-anchor flow for one of the caller's unresolved
  // annotations: the reader switches to "select the new passage" mode.
  onReanchor?(id: string): void;
  // Copies one item or the full list with a document-scoped credential.
  onCopy?: (format: AnnotationCopyFormat, ids?: string[]) => Promise<boolean>;
}

// AnnotationPanel shows the document's shared annotations and the author's editing controls.
export function AnnotationPanel({
  open,
  items,
  statuses,
  loading,
  error,
  selectedId,
  selectionVersion,
  document,
  onLocate,
  onDelete,
  onEditComment,
  onReanchor,
  onCopy,
}: AnnotationPanelProps) {
  const [confirming, setConfirming] = useState(false);
  const [deleteAll, setDeleteAll] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [savingComment, setSavingComment] = useState(false);
  const panelRef = useRef<HTMLElement | null>(null);
  useEffect(() => {
    // A second click on the same highlight must scroll its row again.
    void selectionVersion;
    if (!open || selectedId === null) return;
    const panel = panelRef.current;
    const row = panel?.querySelector<HTMLElement>(
      `[data-annotation-id="${CSS.escape(selectedId)}"]`,
    );
    const viewport = row?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );
    if (row && viewport)
      viewport.scrollTop +=
        row.getBoundingClientRect().top -
        viewport.getBoundingClientRect().top -
        12;
  }, [open, selectedId, selectionVersion]);

  const ownItems = items.filter((item) => item.mine);
  const deletionTargets = deleteAll
    ? ownItems
    : ownItems.filter(
        ({ id }) => statuses[id] === "orphaned" || statuses[id] === "ambiguous",
      );

  async function deleteAnnotations(ids: string | string[]) {
    setDeleting(true);
    try {
      await onDelete(ids);
    } finally {
      setDeleting(false);
    }
  }

  const list = (
    <div className="flex min-h-0 flex-1 flex-col gap-3">
      {error !== null ? (
        <p className="text-destructive px-1 text-sm" role="alert">
          {error}
        </p>
      ) : null}
      {loading ? (
        <p className="text-muted-foreground px-1 text-sm">正在加载标注…</p>
      ) : items.length === 0 ? (
        <p className="text-muted-foreground px-1 py-8 text-center text-sm">
          还没有标注。选中正文后使用「高亮」或「评论」。
        </p>
      ) : (
        <ScrollArea className="min-h-0 flex-1" scrollbarVisibility="scrolling">
          <ol className="flex flex-col gap-2 pr-2">
            {items.map((item) => {
              const status = statuses[item.id] ?? "resolved";
              return (
                <li
                  key={item.id}
                  data-annotation-id={item.id}
                  className={`border-border bg-card rounded-lg border p-2.5 text-sm transition-colors ${
                    selectedId === item.id ? "border-primary/60" : ""
                  }`}
                >
                  <blockquote className="border-border text-muted-foreground border-l-2 pl-2 text-xs leading-relaxed">
                    {item.selector.quote.exact}
                  </blockquote>
                  {editingId === item.id && onEditComment ? (
                    <AnnotationEditor
                      initial={item.comment}
                      autoFocus
                      pending={savingComment}
                      onCancel={() => setEditingId(null)}
                      onSave={(comment) => {
                        setSavingComment(true);
                        void onEditComment(item.id, comment)
                          .then((saved) => {
                            if (saved) setEditingId(null);
                          })
                          .finally(() => setSavingComment(false));
                      }}
                    />
                  ) : item.comment !== "" ? (
                    <p className="mt-1.5 whitespace-pre-wrap">{item.comment}</p>
                  ) : null}
                  <div className="mt-1.5">
                    <AnnotationCommentMeta item={item} />
                  </div>
                  <div className="mt-1.5 flex items-center gap-1">
                    <AnnotationAction
                      label="定位到正文"
                      tooltip={
                        status === "resolved"
                          ? undefined
                          : "此标注无法定位，文档可能已经更新"
                      }
                      disabled={status !== "resolved"}
                      onClick={() => onLocate(item.id)}
                    >
                      <Crosshair aria-hidden="true" />
                    </AnnotationAction>
                    {status !== "resolved" && item.mine && onReanchor ? (
                      <AnnotationAction
                        label="重新关联"
                        tooltip="重新选取正文，重新关联此标注"
                        onClick={() => onReanchor(item.id)}
                      >
                        <LocateFixed aria-hidden="true" />
                      </AnnotationAction>
                    ) : null}
                    <AnnotationCopyButton
                      format="markdown"
                      label="复制此标注（Markdown）"
                      disabled={!onCopy || !document}
                      onCopy={() =>
                        onCopy?.("markdown", [item.id]) ??
                        Promise.resolve(false)
                      }
                    />
                    <AnnotationCopyButton
                      format="json"
                      label="复制此标注（JSON）"
                      disabled={!onCopy || !document}
                      onCopy={() =>
                        onCopy?.("json", [item.id]) ?? Promise.resolve(false)
                      }
                    />
                    {item.mine ? (
                      <>
                        {onEditComment ? (
                          <AnnotationAction
                            label="编辑此评论"
                            onClick={() => setEditingId(item.id)}
                          >
                            <Pencil aria-hidden="true" />
                          </AnnotationAction>
                        ) : null}
                        <AnnotationAction
                          label="删除此标注"
                          disabled={deleting}
                          onClick={() => void deleteAnnotations(item.id)}
                        >
                          <Trash2 aria-hidden="true" />
                        </AnnotationAction>
                      </>
                    ) : null}
                  </div>
                </li>
              );
            })}
          </ol>
        </ScrollArea>
      )}
    </div>
  );

  const header = (
    <div className="flex items-center justify-between gap-2">
      <div className="reader-toc-title mb-0 flex items-center gap-1">
        本页标注
        <span>{items.length}</span>
      </div>
      <div className="flex items-center gap-1">
        <AnnotationCopyButton
          format="markdown"
          label="复制全部标注（Markdown）"
          disabled={!onCopy || !document || items.length === 0}
          onCopy={() => onCopy?.("markdown") ?? Promise.resolve(false)}
        />
        <AnnotationCopyButton
          format="json"
          label="复制全部标注（JSON）"
          disabled={!onCopy || !document || items.length === 0}
          onCopy={() => onCopy?.("json") ?? Promise.resolve(false)}
        />
        <Dialog
          open={confirming}
          onOpenChange={(nextOpen) => {
            setDeleteAll(false);
            setConfirming(nextOpen);
          }}
        >
          <DialogTrigger
            render={
              <AnnotationAction
                label="批量删除标注"
                disabled={loading || deleting || ownItems.length === 0}
              />
            }
          >
            <Trash2 aria-hidden="true" />
          </DialogTrigger>
          <DialogContent
            className="top-1/2 max-w-sm -translate-y-1/2 gap-3 p-4 data-ending-style:-translate-y-1/2 data-starting-style:-translate-y-1/2"
            showCloseButton={false}
          >
            <DialogTitle>批量删除标注</DialogTitle>
            <DialogDescription>
              将删除你在当前文档中{deleteAll ? "的全部" : "无法定位的"}
              {deletionTargets.length} 条标注。
            </DialogDescription>
            <label className="flex items-center gap-2 text-sm">
              <input
                type="checkbox"
                className="accent-primary"
                checked={deleteAll}
                onChange={(event) => setDeleteAll(event.target.checked)}
              />
              删除所有标注
            </label>
            <div className="flex justify-end gap-2">
              <DialogClose render={<Button variant="ghost" size="sm" />}>
                取消
              </DialogClose>
              <Button
                variant="destructive"
                size="sm"
                disabled={deletionTargets.length === 0}
                onClick={() => {
                  setConfirming(false);
                  void deleteAnnotations(deletionTargets.map(({ id }) => id));
                }}
              >
                确认删除
              </Button>
            </div>
          </DialogContent>
        </Dialog>
      </div>
    </div>
  );

  if (!open) {
    return null;
  }
  return (
    <aside
      ref={panelRef}
      className="border-border bg-background reader-annotations flex min-h-0 w-full flex-1 flex-col gap-3 p-3"
      aria-label="标注面板"
    >
      {header}
      {list}
    </aside>
  );
}
