import { Crosshair, LocateFixed, Pencil, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AnnotationColor, AnnotationItem } from "@/api";
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
import type { AnnotationDeleteResult } from "@/hooks/use-annotations";
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
  onDelete(ids: string | string[]): Promise<AnnotationDeleteResult>;
  onUpdateAnnotation?(
    id: string,
    comment: string,
    color: AnnotationColor,
  ): Promise<boolean>;
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
  onUpdateAnnotation,
  onReanchor,
  onCopy,
}: AnnotationPanelProps) {
  const [confirming, setConfirming] = useState(false);
  const [deleteAll, setDeleteAll] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [deleteErrors, setDeleteErrors] = useState<Map<string, string>>(
    new Map(),
  );
  const [batchResult, setBatchResult] = useState<{
    deletedCount: number;
    failures: AnnotationDeleteResult["failures"];
  } | null>(null);
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

  async function deleteRow(id: string) {
    setDeleting(true);
    try {
      const { failures } = await onDelete(id);
      setDeleteErrors((current) => {
        const next = new Map(current);
        const failure = failures[0];
        if (failure) next.set(id, failure.message);
        else next.delete(id);
        return next;
      });
    } finally {
      setDeleting(false);
    }
  }

  async function deleteBatch() {
    const ids = batchResult
      ? batchResult.failures.map(({ id }) => id)
      : deletionTargets.map(({ id }) => id);
    setDeleting(true);
    try {
      const result = await onDelete(ids);
      if (result.failures.length === 0) {
        setConfirming(false);
        return;
      }
      setBatchResult({
        deletedCount:
          (batchResult?.deletedCount ?? 0) + result.deletedIds.length,
        failures: result.failures,
      });
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
          还没有标注。选中正文后点击「标注」。
        </p>
      ) : (
        <ScrollArea className="min-h-0 flex-1" scrollbarVisibility="scrolling">
          <ol className="flex flex-col gap-2 pr-2">
            {items.map((item) => {
              const status = statuses[item.id] ?? "resolved";
              const deleteError = deleteErrors.get(item.id);
              return (
                <li
                  key={item.id}
                  data-annotation-id={item.id}
                  data-annotation-color={item.color}
                  className={`border-border bg-card rounded-lg border p-2.5 text-sm transition-colors ${
                    selectedId === item.id ? "border-primary/60" : ""
                  }`}
                >
                  <blockquote className="border-border text-muted-foreground border-l-2 pl-2 text-xs leading-relaxed">
                    {item.selector.quote.exact}
                  </blockquote>
                  {editingId === item.id && onUpdateAnnotation ? (
                    <AnnotationEditor
                      initial={item.comment}
                      initialColor={item.color}
                      autoFocus
                      pending={savingComment}
                      onCancel={() => setEditingId(null)}
                      onSave={(comment, color) => {
                        setSavingComment(true);
                        void onUpdateAnnotation(item.id, comment, color)
                          .then((saved) => {
                            if (saved) setEditingId(null);
                          })
                          .finally(() => setSavingComment(false));
                      }}
                    />
                  ) : (
                    <>
                      {item.comment !== "" ? (
                        <p className="mt-1.5 whitespace-pre-wrap">
                          {item.comment}
                        </p>
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
                            onCopy?.("json", [item.id]) ??
                            Promise.resolve(false)
                          }
                        />
                        {item.mine ? (
                          <>
                            {onUpdateAnnotation ? (
                              <AnnotationAction
                                label="编辑此评论"
                                onClick={() => setEditingId(item.id)}
                              >
                                <Pencil aria-hidden="true" />
                              </AnnotationAction>
                            ) : null}
                            <AnnotationAction
                              label={
                                deleteError ? "重试删除此标注" : "删除此标注"
                              }
                              disabled={deleting}
                              onClick={() => void deleteRow(item.id)}
                            >
                              <Trash2 aria-hidden="true" />
                            </AnnotationAction>
                          </>
                        ) : null}
                      </div>
                      {deleteError ? (
                        <p
                          className="text-destructive mt-1 text-xs"
                          role="alert"
                        >
                          {deleteError}
                        </p>
                      ) : null}
                    </>
                  )}
                </li>
              );
            })}
          </ol>
        </ScrollArea>
      )}
    </div>
  );

  const header = (
    <div className="reader-rail-header">
      <div className="reader-rail-title">
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
            if (deleting) return;
            setDeleteAll(false);
            setBatchResult(null);
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
              {batchResult
                ? "重试时只会删除本次失败的标注。"
                : deleteAll
                  ? `将删除你在当前文档中的全部 ${deletionTargets.length} 条标注。`
                  : `将删除你在当前文档中无法定位的 ${deletionTargets.length} 条标注。`}
            </DialogDescription>
            {batchResult ? (
              <div className="text-destructive space-y-1 text-sm" role="alert">
                <p>
                  已删除 {batchResult.deletedCount} 条，
                  {batchResult.failures.length} 条删除失败。
                </p>
                {[
                  ...new Set(
                    batchResult.failures.map(({ message }) => message),
                  ),
                ].map((message) => (
                  <p key={message}>{message}</p>
                ))}
              </div>
            ) : (
              <label className="flex items-center gap-2 text-sm">
                <input
                  type="checkbox"
                  className="accent-primary"
                  checked={deleteAll}
                  disabled={deleting}
                  onChange={(event) => setDeleteAll(event.target.checked)}
                />
                删除我在当前文档中的全部标注
              </label>
            )}
            {deleting ? (
              <p className="text-muted-foreground text-sm" role="status">
                正在删除标注…
              </p>
            ) : null}
            <div className="flex justify-end gap-2">
              <DialogClose
                render={
                  <Button variant="ghost" size="sm" disabled={deleting} />
                }
              >
                {batchResult ? "关闭" : "取消"}
              </DialogClose>
              <Button
                variant="destructive"
                size="sm"
                disabled={
                  deleting || (!batchResult && deletionTargets.length === 0)
                }
                onClick={() => void deleteBatch()}
              >
                {batchResult ? "重试失败项" : "确认删除"}
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
