import { useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/button";

export interface AnnotationEditorProps {
  // Initial comment text (empty for a fresh annotation).
  initial?: string;
  autoFocus?: boolean;
  pending: boolean;
  placeholder?: string;
  onCancel(): void;
  onSave(comment: string): void;
}

// AnnotationEditor is the one-box comment editor shared by the selection
// toolbar and the panel. It is plain text — never markup — and keeps the
// entered text across a failed save so nothing is retyped.
export function AnnotationEditor({
  initial = "",
  autoFocus = false,
  pending,
  placeholder = "添加评论…",
  onCancel,
  onSave,
}: AnnotationEditorProps) {
  const [comment, setComment] = useState(initial);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    if (autoFocus) {
      textareaRef.current?.focus();
    }
  }, [autoFocus]);

  return (
    <div className="flex flex-col gap-1.5 p-1">
      <textarea
        ref={textareaRef}
        className="bg-background border-input placeholder:text-muted-foreground focus-visible:border-ring focus-visible:ring-ring/50 min-h-16 rounded-lg border px-2 py-1.5 text-sm outline-none focus-visible:ring-2"
        value={comment}
        placeholder={placeholder}
        disabled={pending}
        onChange={(event) => setComment(event.target.value)}
        onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.stopPropagation();
            onCancel();
          }
          if (event.key === "Enter" && (event.metaKey || event.ctrlKey)) {
            onSave(comment);
          }
        }}
      />
      <div className="flex justify-end gap-1">
        <Button variant="ghost" size="xs" disabled={pending} onClick={onCancel}>
          取消
        </Button>
        <Button size="xs" disabled={pending} onClick={() => onSave(comment)}>
          保存
        </Button>
      </div>
    </div>
  );
}
