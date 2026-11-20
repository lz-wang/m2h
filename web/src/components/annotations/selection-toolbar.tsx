import { Highlighter, LocateFixed, MessageSquarePlus } from "lucide-react";
import {
  type RefObject,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { Button } from "@/components/ui/button";
import {
  DRAFT_ANNOTATION_HIGHLIGHT_NAME,
  HighlightLayer,
} from "@/lib/annotation-highlight";
import {
  type AnnotationSelector,
  selectorFromRange,
  type TextIndex,
} from "@/lib/annotation-selector";
import { AnnotationEditor } from "./annotation-editor";
import {
  type AnnotationCardBounds,
  AnnotationFloatingCard,
  annotationCardHeight,
  annotationCardWidth,
} from "./annotation-floating-card";

export interface SelectionToolbarProps {
  // The rendered body root; selections outside it never open the toolbar.
  bodyRef: RefObject<HTMLElement | null>;
  // The live text index of the current body (kept fresh by the app).
  indexRef: RefObject<TextIndex | null>;
  // Saves an annotation for the given selector; resolves false on failure
  // (the toolbar then stays put, selection intact, so nothing is lost).
  onAnnotate: (
    selector: AnnotationSelector,
    comment: string,
    pinnedBounds?: AnnotationCardBounds,
  ) => Promise<boolean>;
  // Re-anchor mode: while pending, the toolbar serves the human
  // re-association flow — the next valid selection becomes the
  // annotation's new anchor instead of a new annotation.
  reanchorPending?: { quote: string } | null;
  // Confirms the re-anchor with the freshly selected text.
  onReanchor?: (selector: AnnotationSelector) => Promise<boolean>;
  // Aborts re-anchor mode (Escape).
  onReanchorCancel?: () => void;
  disabled?: boolean;
}

interface ToolbarPosition {
  top: number;
  left: number;
}

// SelectionToolbar is the floating 高亮/评论 pill that appears on a valid
// body selection. Only canonical-content selections qualify: a range the
// text index cannot address (empty, collapsed, or touching UI decorations
// and generated visuals) never shows it.
export function SelectionToolbar({
  bodyRef,
  indexRef,
  onAnnotate,
  reanchorPending = null,
  onReanchor,
  onReanchorCancel,
  disabled = false,
}: SelectionToolbarProps) {
  const [selector, setSelector] = useState<AnnotationSelector | null>(null);
  const [position, setPosition] = useState<ToolbarPosition | null>(null);
  const [commentPosition, setCommentPosition] =
    useState<ToolbarPosition | null>(null);
  const commenting = commentPosition !== null;
  const [commentPinned, setCommentPinned] = useState(false);
  const commentBounds = useRef<AnnotationCardBounds | null>(null);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  // The live DOM selection is re-read on demand; only its derived selector
  // and rectangle live in state.
  const rangeRef = useRef<Range | null>(null);

  // Leaving re-anchor mode discards any pending toolbar state.
  useEffect(() => {
    if (reanchorPending === null) {
      return;
    }
    setCommentPosition(null);
    setFailure(null);
  }, [reanchorPending]);

  useEffect(() => {
    if (disabled) {
      setSelector(null);
      setPosition(null);
      setCommentPosition(null);
    }
  }, [disabled]);

  const updateFromSelection = useCallback(() => {
    if (disabled) {
      return;
    }
    const body = bodyRef.current;
    const index = indexRef.current;
    const selection = document.getSelection();
    if (
      body === null ||
      index === null ||
      selection === null ||
      selection.rangeCount === 0
    ) {
      return;
    }
    const range = selection.getRangeAt(0);
    if (range.collapsed || !body.contains(range.commonAncestorContainer)) {
      return;
    }
    const candidate = selectorFromRange(index, range);
    if (candidate === null) {
      return;
    }
    rangeRef.current = range.cloneRange();
    setSelector(candidate);
    setFailure(null);
    // Some engines (and test DOMs) lack Range.getBoundingClientRect; a
    // zero rectangle only means the pill parks at the top-left — the
    // annotation itself is unaffected.
    let top = 8;
    let center = 90;
    if (typeof range.getBoundingClientRect === "function") {
      const rectangle = range.getBoundingClientRect();
      top = Math.max(rectangle.top - 44, 8);
      center = rectangle.left + rectangle.width / 2;
    }
    const left = Math.min(Math.max(center, 90), window.innerWidth - 90);
    setPosition({ top, left });
  }, [bodyRef, indexRef, disabled]);

  useEffect(() => {
    const handleSelectionChange = () => {
      // The editor's own textarea drives selectionchange too; ignore those.
      if (commenting || saving) {
        return;
      }
      updateFromSelection();
      if (document.getSelection()?.isCollapsed === true) {
        setSelector(null);
        setPosition(null);
      }
    };
    document.addEventListener("selectionchange", handleSelectionChange);
    return () =>
      document.removeEventListener("selectionchange", handleSelectionChange);
  }, [updateFromSelection, commenting, saving]);

  useLayoutEffect(() => {
    if (disabled || !commenting || selector === null) return;
    const layer = new HighlightLayer(DRAFT_ANNOTATION_HIGHLIGHT_NAME);
    layer.attach(indexRef.current);
    layer.render([selector.position]);
    return () => layer.release();
  }, [commenting, disabled, indexRef, selector]);

  const cancelComment = useCallback(() => {
    if (saving) return;
    const range = rangeRef.current;
    if (
      range !== null &&
      bodyRef.current?.contains(range.commonAncestorContainer)
    ) {
      if (document.activeElement instanceof HTMLElement)
        document.activeElement.blur();
      const selection = document.getSelection();
      selection?.removeAllRanges();
      selection?.addRange(range);
    }
    setCommentPosition(null);
    setFailure(null);
  }, [bodyRef, saving]);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && commenting) {
        cancelComment();
        return;
      }
      // Escape also serves the re-anchor flow's banner promise while no
      // text is selected yet — that is most of the flow's lifetime.
      if (
        event.key === "Escape" &&
        (selector !== null || reanchorPending !== null)
      ) {
        setSelector(null);
        setPosition(null);
        setCommentPosition(null);
        if (reanchorPending !== null) {
          onReanchorCancel?.();
        }
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [selector, reanchorPending, onReanchorCancel, commenting, cancelComment]);

  if (selector === null || position === null || disabled) {
    return null;
  }

  const save = async (comment: string) => {
    if (selector === null || saving) {
      return;
    }
    setSaving(true);
    setFailure(null);
    const bounds = commenting && commentPinned ? commentBounds.current : null;
    const saved = bounds
      ? await onAnnotate(selector, comment, bounds)
      : await onAnnotate(selector, comment);
    setSaving(false);
    if (saved) {
      setSelector(null);
      setPosition(null);
      setCommentPosition(null);
      document.getSelection()?.removeAllRanges();
    } else {
      setFailure("保存失败，可重试");
    }
  };

  const confirmReanchor = async () => {
    if (selector === null || saving || onReanchor === undefined) {
      return;
    }
    setSaving(true);
    setFailure(null);
    const saved = await onReanchor(selector);
    setSaving(false);
    if (saved) {
      setSelector(null);
      setPosition(null);
      document.getSelection()?.removeAllRanges();
    } else {
      setFailure("保存失败，可重试");
    }
  };

  if (reanchorPending !== null && onReanchor !== undefined) {
    return (
      <div
        className="bg-popover text-popover-foreground border-border fixed z-40 flex flex-col gap-1 rounded-lg border p-1 shadow-lg"
        style={{
          top: position.top,
          left: position.left,
          transform: "translateX(-50%)",
        }}
        role="toolbar"
        aria-label="重新关联标注"
      >
        <p
          className="text-muted-foreground max-w-60 truncate px-2 pt-1 text-xs"
          title={reanchorPending.quote}
        >
          重新关联：{reanchorPending.quote}
        </p>
        <div className="flex items-center gap-0.5">
          <Button
            variant="ghost"
            size="xs"
            disabled={saving}
            onClick={() => void confirmReanchor()}
          >
            <LocateFixed aria-hidden="true" />
            关联到此选区
          </Button>
        </div>
        {failure !== null ? (
          <p className="text-destructive px-2 pb-1 text-xs" role="alert">
            {failure}
          </p>
        ) : null}
      </div>
    );
  }

  if (commentPosition !== null) {
    return (
      <AnnotationFloatingCard
        left={commentPosition.left}
        top={commentPosition.top}
        label="新建评论"
        closeDisabled={saving}
        pinned={commentPinned}
        onPinnedChange={(pinned, bounds) => {
          setCommentPinned(pinned);
          commentBounds.current = bounds;
        }}
        onAdjust={(bounds) => {
          commentBounds.current = bounds;
        }}
        onClose={cancelComment}
      >
        <AnnotationEditor
          autoFocus
          pending={saving}
          onCancel={cancelComment}
          onSave={(comment) => void save(comment)}
        />
        {failure !== null ? (
          <p className="text-destructive px-2 pb-1 text-xs" role="alert">
            {failure}
          </p>
        ) : null}
      </AnnotationFloatingCard>
    );
  }

  function openComment() {
    if (position === null) return;
    setCommentPinned(false);
    commentBounds.current = null;
    const rect = rangeRef.current?.getBoundingClientRect?.();
    const below = (rect?.bottom ?? position.top) + 12;
    const above = (rect?.top ?? position.top) - annotationCardHeight - 12;
    setCommentPosition({
      left: position.left - annotationCardWidth / 2,
      top:
        below + annotationCardHeight <= window.innerHeight || above < 8
          ? below
          : above,
    });
    setFailure(null);
  }

  return (
    <div
      className="bg-popover text-popover-foreground border-border fixed z-40 flex flex-col gap-1 rounded-lg border p-1 shadow-lg"
      style={{
        top: position.top,
        left: position.left,
        transform: "translateX(-50%)",
      }}
      role="toolbar"
      aria-label="标注选中文本"
    >
      <div className="flex items-center gap-0.5">
        <Button
          variant="ghost"
          size="xs"
          disabled={saving}
          onClick={() => void save("")}
        >
          <Highlighter aria-hidden="true" />
          高亮
        </Button>
        <Button
          variant="ghost"
          size="xs"
          disabled={saving}
          onClick={openComment}
        >
          <MessageSquarePlus aria-hidden="true" />
          评论
        </Button>
      </div>
      {failure !== null ? (
        <p className="text-destructive px-2 pb-1 text-xs" role="alert">
          {failure}
        </p>
      ) : null}
    </div>
  );
}
