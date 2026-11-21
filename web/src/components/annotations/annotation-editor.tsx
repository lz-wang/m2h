import { useEffect, useRef, useState } from "react";
import {
  ANNOTATION_COLORS,
  type AnnotationColor,
  DEFAULT_ANNOTATION_COLOR,
} from "@/api";

import { Button } from "@/components/ui/button";

export interface AnnotationEditorProps {
  // Initial comment text (empty for a fresh annotation).
  initial?: string;
  autoFocus?: boolean;
  initialColor?: AnnotationColor;
  onColorChange?(color: AnnotationColor): void;
  pending: boolean;
  placeholder?: string;
  onCancel(): void;
  onSave(comment: string, color: AnnotationColor): void;
}

const colorLabels: Record<AnnotationColor, string> = {
  blue: "蓝色",
  green: "绿色",
  amber: "琥珀色",
};

// AnnotationEditor is the one-box comment editor shared by the selection
// toolbar and the panel. It is plain text — never markup — and keeps the
// entered text across a failed save so nothing is retyped.
export function AnnotationEditor({
  initial = "",
  autoFocus = false,
  initialColor = DEFAULT_ANNOTATION_COLOR,
  onColorChange,
  pending,
  placeholder = "添加评论…",
  onCancel,
  onSave,
}: AnnotationEditorProps) {
  const [comment, setComment] = useState(initial);
  const [color, setColor] = useState(initialColor);
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
        data-annotation-color={color}
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
            onSave(comment, color);
          }
        }}
      />
      <div className="flex items-center gap-2">
        <fieldset className="flex items-center gap-0.5" aria-label="标注颜色">
          {ANNOTATION_COLORS.map((option) => (
            <Button
              key={option}
              type="button"
              variant="ghost"
              size="icon-xs"
              aria-label={colorLabels[option]}
              aria-pressed={option === color}
              disabled={pending}
              data-annotation-color={option}
              onMouseDown={(event) => event.preventDefault()}
              onClick={() => {
                setColor(option);
                onColorChange?.(option);
              }}
            >
              <span className="annotation-color-swatch" />
            </Button>
          ))}
        </fieldset>
        <div className="ml-auto flex gap-1">
          <Button
            variant="ghost"
            size="xs"
            disabled={pending}
            onClick={onCancel}
          >
            取消
          </Button>
          <Button
            size="xs"
            disabled={pending}
            onClick={() => onSave(comment, color)}
          >
            保存
          </Button>
        </div>
      </div>
    </div>
  );
}
