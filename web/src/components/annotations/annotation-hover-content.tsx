import { ArrowLeft, Pencil, Trash2 } from "lucide-react";
import { useState } from "react";
import type { AnnotationItem } from "@/api";
import { Button } from "@/components/ui/button";
import { AnnotationAction } from "./annotation-action";
import { AnnotationCommentMeta } from "./annotation-comment-meta";
import {
  AnnotationCopyButton,
  type AnnotationCopyFormat,
} from "./annotation-copy-button";
import { AnnotationEditor } from "./annotation-editor";

export function AnnotationHoverList({
  items,
  onSelect,
}: {
  items: AnnotationItem[];
  onSelect(id: string): void;
}) {
  return (
    <>
      <p className="text-muted-foreground px-2 pt-1 pb-0.5 text-xs">
        {items.length} 条标注
      </p>
      <ul
        aria-label="重叠标注"
        className="divide-border min-h-0 flex-1 divide-y overflow-auto pr-1"
      >
        {items.map((item) => (
          <li key={item.id}>
            <Button
              variant="ghost"
              className="h-auto w-full flex-col items-start gap-1 px-2 py-1.5 text-left font-normal whitespace-normal"
              onClick={() => onSelect(item.id)}
            >
              <span className="line-clamp-1 [overflow-wrap:anywhere]">
                {item.comment || item.selector.quote.exact}
              </span>
              <AnnotationCommentMeta item={item} />
            </Button>
          </li>
        ))}
      </ul>
    </>
  );
}

export function AnnotationHoverDetail({
  item,
  editing,
  onEditingChange,
  onBack,
  onCopy,
  onSave,
  onDelete,
  deleting,
  deleteError,
}: {
  item: AnnotationItem;
  editing: boolean;
  onEditingChange(editing: boolean): void;
  onBack?: () => void;
  onCopy?: (format: AnnotationCopyFormat) => Promise<boolean>;
  onSave(comment: string): Promise<boolean>;
  onDelete(): void;
  deleting: boolean;
  deleteError?: string;
}) {
  const [saving, setSaving] = useState(false);

  return (
    <>
      {onBack ? (
        <Button
          variant="ghost"
          size="xs"
          className="mb-1 self-start"
          disabled={deleting}
          onClick={onBack}
        >
          <ArrowLeft aria-hidden="true" />
          返回标注列表
        </Button>
      ) : null}
      {editing ? (
        <AnnotationEditor
          initial={item.comment}
          color={item.color}
          autoFocus
          pending={saving}
          onCancel={() => onEditingChange(false)}
          onSave={(comment) => {
            setSaving(true);
            void onSave(comment)
              .then((saved) => {
                if (saved) onEditingChange(false);
              })
              .finally(() => setSaving(false));
          }}
        />
      ) : (
        // biome-ignore lint/a11y/noStaticElementInteractions: The edit button provides the keyboard equivalent of this shortcut.
        <div
          className="annotation-comment-box"
          data-annotation-color={item.color}
          onDoubleClick={() => {
            if (item.mine && !deleting) onEditingChange(true);
          }}
        >
          {item.comment || "无评论"}
        </div>
      )}
      <div className="mt-1 px-1">
        <AnnotationCommentMeta item={item} />
      </div>
      {deleteError ? (
        <p className="text-destructive mt-1 px-1 text-xs" role="alert">
          {deleteError}
        </p>
      ) : null}
      {editing ? null : (
        <div className="annotation-hover-actions">
          <AnnotationCopyButton
            format="markdown"
            label="复制标注（Markdown）"
            disabled={!onCopy}
            onCopy={() => onCopy?.("markdown") ?? Promise.resolve(false)}
          />
          <AnnotationCopyButton
            format="json"
            label="复制标注（JSON）"
            disabled={!onCopy}
            onCopy={() => onCopy?.("json") ?? Promise.resolve(false)}
          />
          {item.mine ? (
            <>
              <AnnotationAction
                label="编辑评论"
                disabled={deleting}
                onClick={() => onEditingChange(true)}
              >
                <Pencil aria-hidden="true" />
              </AnnotationAction>
              <AnnotationAction
                className="text-destructive hover:text-destructive"
                label={deleteError ? "重试删除标注" : "删除标注"}
                disabled={deleting}
                onClick={onDelete}
              >
                <Trash2 aria-hidden="true" />
              </AnnotationAction>
            </>
          ) : null}
        </div>
      )}
    </>
  );
}
