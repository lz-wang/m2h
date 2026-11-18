import { Crosshair, LocateFixed, Pencil, Trash2 } from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AnnotationItem } from "@/api";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import type { ExportDocument } from "@/lib/annotation-export";
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
  onDelete(id: string): void;
  onEditComment?(id: string, comment: string): Promise<boolean>;
  // Starts the human re-anchor flow for one of the caller's unresolved
  // annotations: the reader switches to "select the new passage" mode.
  onReanchor?(id: string): void;
  // Copies one item or the full list with a document-scoped credential.
  onCopy?: (format: AnnotationCopyFormat, ids?: string[]) => Promise<boolean>;
}

// AnnotationPanel is the right-hand annotation rail:
// the document's annotation list with locate, copy, delete and whole-set
// export. Other users' annotations are read-only; the panel states that
// with a badge instead of hiding entries — review is a shared view.
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
  const [confirming, setConfirming] = useState<string | null>(null);
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
                  <div className="text-muted-foreground mt-1.5 flex items-center gap-2 text-xs">
                    <span>{item.authorName}</span>
                    <span aria-hidden="true">·</span>
                    <time>{new Date(item.createdAt).toLocaleString()}</time>
                    {status === "orphaned" ? (
                      <span className="bg-destructive/10 text-destructive rounded px-1.5 py-0.5">
                        未定位
                      </span>
                    ) : null}
                    {status === "ambiguous" ? (
                      <span className="bg-muted text-muted-foreground rounded px-1.5 py-0.5">
                        位置不确定
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-1.5 flex items-center gap-1">
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      aria-label="定位到正文"
                      title="定位到正文"
                      onClick={() => onLocate(item.id)}
                    >
                      <Crosshair aria-hidden="true" />
                    </Button>
                    {status !== "resolved" && item.mine && onReanchor ? (
                      <Button
                        variant="ghost"
                        size="icon-xs"
                        aria-label="重新关联"
                        title="重新选取正文，重新关联此标注"
                        onClick={() => onReanchor(item.id)}
                      >
                        <LocateFixed aria-hidden="true" />
                      </Button>
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
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            aria-label="编辑此评论"
                            title="编辑此评论"
                            onClick={() => setEditingId(item.id)}
                          >
                            <Pencil aria-hidden="true" />
                          </Button>
                        ) : null}
                        {confirming === item.id ? (
                          <>
                            <Button
                              variant="destructive"
                              size="xs"
                              onClick={() => {
                                onDelete(item.id);
                                setConfirming(null);
                              }}
                            >
                              确认删除
                            </Button>
                            <Button
                              variant="ghost"
                              size="xs"
                              onClick={() => setConfirming(null)}
                            >
                              取消
                            </Button>
                          </>
                        ) : (
                          <Button
                            variant="ghost"
                            size="icon-xs"
                            aria-label="删除此标注"
                            title="删除此标注"
                            onClick={() => setConfirming(item.id)}
                          >
                            <Trash2 aria-hidden="true" />
                          </Button>
                        )}
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
