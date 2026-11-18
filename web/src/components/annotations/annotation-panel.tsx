import {
  Bot,
  Check,
  ClipboardCopy,
  Crosshair,
  FileJson,
  FileText,
  LoaderCircle,
  LocateFixed,
  MessageSquare,
  Trash2,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import type { AnnotationItem } from "@/api";
import { Button } from "@/components/ui/button";
import { ScrollArea } from "@/components/ui/scroll-area";
import {
  type ExportAnnotation,
  type ExportDocument,
  exportAnnotationsJSON,
  exportAnnotationsMarkdown,
  exportSingleAnnotationMarkdown,
} from "@/lib/annotation-export";
import { copyText } from "@/lib/clipboard";

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
  // Starts the human re-anchor flow for one of the caller's unresolved
  // annotations: the reader switches to "select the new passage" mode.
  onReanchor?(id: string): void;
  // Optional: mints a temporary document-scoped token and copies an
  // agent-ready review packet to the clipboard (authenticated runs with
  // tokens enabled).
  onCopyForAgent?: () => Promise<boolean>;
}

// AnnotationPanel is the right-hand annotation rail (a Sheet on mobile):
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
  onReanchor,
  onCopyForAgent,
}: AnnotationPanelProps) {
  const [confirming, setConfirming] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [agentPending, setAgentPending] = useState(false);
  const [agentNotice, setAgentNotice] = useState<string | null>(null);
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

  const asExport = (item: AnnotationItem): ExportAnnotation => ({
    id: item.id,
    documentPath: document?.path ?? "",
    documentHash: item.documentHash,
    authorName: item.authorName,
    comment: item.comment,
    selector: {
      quote: {
        exact: item.selector.quote.exact,
        prefix: item.selector.quote.prefix,
        suffix: item.selector.quote.suffix,
      },
      position: item.selector.position,
      headingId: item.selector.headingId,
      tabId: item.selector.tabId,
      blockHint: item.selector.blockHint,
      version: 1,
    },
    status: statuses[item.id] ?? "resolved",
    createdAt: item.createdAt,
  });

  const copy = (text: string) => {
    void copyText(text).then((ok) => {
      setCopied(ok);
      window.setTimeout(() => setCopied(false), 1500);
    });
  };

  const copyForAgent = async () => {
    if (onCopyForAgent === undefined || agentPending) {
      return;
    }
    setAgentPending(true);
    setAgentNotice(null);
    const ok = await onCopyForAgent();
    setAgentPending(false);
    setAgentNotice(ok ? "已复制评审材料与临时凭证" : "复制失败，请重试");
    window.setTimeout(() => setAgentNotice(null), 2500);
  };

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
                  {item.comment !== "" ? (
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
                    <Button
                      variant="ghost"
                      size="icon-xs"
                      aria-label="复制此标注"
                      title="复制此标注"
                      onClick={() =>
                        document !== null &&
                        copy(
                          exportSingleAnnotationMarkdown(
                            document,
                            asExport(item),
                          ),
                        )
                      }
                    >
                      <ClipboardCopy aria-hidden="true" />
                    </Button>
                    {item.mine ? (
                      confirming === item.id ? (
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
                      )
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
      <div className="flex items-center gap-2 font-medium">
        <MessageSquare aria-hidden="true" className="size-4" />
        标注
        <span className="text-muted-foreground text-xs font-normal">
          {items.length}
        </span>
      </div>
      <div className="flex items-center gap-1">
        {agentNotice !== null ? (
          <span className="text-muted-foreground inline-flex items-center gap-1 text-xs">
            <Check aria-hidden="true" className="size-3" /> {agentNotice}
          </span>
        ) : null}
        {copied ? (
          <span className="text-muted-foreground inline-flex items-center gap-1 text-xs">
            <Check aria-hidden="true" className="size-3" /> 已复制
          </span>
        ) : null}
        {onCopyForAgent !== undefined && document !== null ? (
          <Button
            variant="ghost"
            size="icon-xs"
            aria-label="复制给 Agent"
            title="复制给 Agent（临时只读凭证 + 评审意见）"
            disabled={agentPending}
            onClick={() => void copyForAgent()}
          >
            {agentPending ? (
              <LoaderCircle aria-hidden="true" className="animate-spin" />
            ) : (
              <Bot aria-hidden="true" />
            )}
          </Button>
        ) : null}
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label="复制全部标注（Markdown）"
          title="复制全部标注（Markdown）"
          disabled={document === null || items.length === 0}
          onClick={() =>
            document !== null &&
            copy(exportAnnotationsMarkdown(document, items.map(asExport)))
          }
        >
          <FileText aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="icon-xs"
          aria-label="复制全部标注（JSON）"
          title="复制全部标注（JSON）"
          disabled={document === null || items.length === 0}
          onClick={() =>
            document !== null &&
            copy(exportAnnotationsJSON(document, items.map(asExport)))
          }
        >
          <FileJson aria-hidden="true" />
        </Button>
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
