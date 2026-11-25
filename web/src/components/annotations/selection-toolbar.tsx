import { Highlighter, LocateFixed } from "lucide-react";
import {
  type RefObject,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { type AnnotationColor, DEFAULT_ANNOTATION_COLOR } from "@/api";
import { Button } from "@/components/ui/button";
import { draftHighlightName, HighlightLayer } from "@/lib/annotation-highlight";
import {
  type AnnotationSelector,
  selectorFromRange,
  type TextIndex,
} from "@/lib/annotation-selector";
import { watchReaderScroll } from "@/lib/reader-scroll";
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
  readerToolbarRef?: RefObject<HTMLElement | null>;
  // The live text index of the current body (kept fresh by the app).
  indexRef: RefObject<TextIndex | null>;
  // Saves an annotation for the given selector; resolves false on failure
  // (the editor then keeps the draft and its selected text).
  onAnnotate: (
    selector: AnnotationSelector,
    comment: string,
    color: AnnotationColor,
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
  bottom: number;
  left: number;
  above: boolean;
}

// SelectionToolbar is the floating 标注 button that appears on a valid
// body selection. Only canonical-content selections qualify: a range the
// text index cannot address (empty, collapsed, or touching UI decorations
// and generated visuals) never shows it.
export function SelectionToolbar({
  bodyRef,
  readerToolbarRef,
  indexRef,
  onAnnotate,
  reanchorPending = null,
  onReanchor,
  onReanchorCancel,
  disabled = false,
}: SelectionToolbarProps) {
  const [selector, setSelector] = useState<AnnotationSelector | null>(null);
  const [color, setColor] = useState<AnnotationColor>(DEFAULT_ANNOTATION_COLOR);
  const [position, setPosition] = useState<ToolbarPosition | null>(null);
  const [commentPosition, setCommentPosition] = useState<Pick<
    ToolbarPosition,
    "top" | "left"
  > | null>(null);
  const commenting = commentPosition !== null;
  const [commentPinned, setCommentPinned] = useState(false);
  const commentBounds = useRef<AnnotationCardBounds | null>(null);
  const [saving, setSaving] = useState(false);
  const [failure, setFailure] = useState<string | null>(null);
  // Keep the range while its text is offscreen and while editing a draft.
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
      rangeRef.current = null;
      setSelector(null);
      setPosition(null);
      setCommentPosition(null);
    }
  }, [disabled]);

  const updatePosition = useCallback(() => {
    const range = rangeRef.current;
    if (
      range === null ||
      !bodyRef.current?.contains(range.commonAncestorContainer)
    ) {
      setPosition(null);
      return;
    }

    const visibleTop = Math.max(
      readerToolbarRef?.current?.getBoundingClientRect().bottom ?? 0,
      0,
    );
    const viewportWidth =
      document.documentElement.clientWidth || window.innerWidth;
    const viewportHeight =
      document.documentElement.clientHeight || window.innerHeight;
    // Use a visible fragment so a wrapped selection can follow the portion
    // that is still on screen, including when it moves under the sticky header.
    for (const rect of range.getClientRects()) {
      const top = Math.max(visibleTop, rect.top);
      const bottom = Math.min(viewportHeight, rect.bottom);
      const left = Math.max(0, rect.left);
      const right = Math.min(viewportWidth, rect.right);
      if (top >= bottom || left >= right) continue;

      const inset = Math.min(90, viewportWidth / 2);
      const center = Math.min(
        Math.max((left + right) / 2, inset),
        viewportWidth - inset,
      );
      const above = top - visibleTop >= (reanchorPending === null ? 44 : 76);
      setPosition({ top, bottom, left: center, above });
      return;
    }
    setPosition(null);
  }, [bodyRef, readerToolbarRef, reanchorPending]);

  useEffect(() => {
    const handleSelectionChange = () => {
      // The editor's own textarea drives selectionchange too; ignore those.
      if (disabled || commenting || saving) return;
      const body = bodyRef.current;
      const index = indexRef.current;
      const selection = document.getSelection();
      const range = selection?.rangeCount ? selection.getRangeAt(0) : null;
      const candidate =
        index &&
        range &&
        !range.collapsed &&
        body?.contains(range.commonAncestorContainer)
          ? selectorFromRange(index, range)
          : null;
      if (candidate === null || range === null) {
        rangeRef.current = null;
        setSelector(null);
      } else {
        rangeRef.current = range.cloneRange();
        setSelector(candidate);
        setFailure(null);
      }
      updatePosition();
    };
    document.addEventListener("selectionchange", handleSelectionChange);
    return () =>
      document.removeEventListener("selectionchange", handleSelectionChange);
  }, [bodyRef, indexRef, updatePosition, disabled, commenting, saving]);

  const hasSelection = selector !== null;
  useEffect(() => {
    if (!hasSelection || disabled || commenting || saving) return;
    const stopWatchingScroll = watchReaderScroll(bodyRef, updatePosition);
    window.addEventListener("resize", updatePosition);
    return () => {
      stopWatchingScroll();
      window.removeEventListener("resize", updatePosition);
    };
  }, [bodyRef, updatePosition, hasSelection, disabled, commenting, saving]);

  useLayoutEffect(() => {
    if (disabled || !commenting || selector === null) return;
    const layer = new HighlightLayer(draftHighlightName(color), 1);
    const body = bodyRef.current;
    const index = indexRef.current;
    layer.attach(index);
    layer.render([selector.position]);
    const refreshDraft = () => {
      rangeRef.current = index?.rangeFromSpan(selector.position) ?? null;
      layer.renderRanges(rangeRef.current ? [rangeRef.current] : []);
    };
    body?.addEventListener("afterSort", refreshDraft, true);
    return () => {
      body?.removeEventListener("afterSort", refreshDraft, true);
      layer.release();
    };
  }, [bodyRef, color, commenting, disabled, indexRef, selector]);

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
    updatePosition();
    setCommentPosition(null);
    setFailure(null);
  }, [bodyRef, saving, updatePosition]);

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
        rangeRef.current = null;
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

  if (selector === null || disabled) {
    return null;
  }

  const toolbarStyle =
    position === null
      ? undefined
      : {
          top: position.above ? position.top - 8 : position.bottom + 8,
          left: position.left,
          transform: position.above
            ? "translate(-50%, -100%)"
            : "translateX(-50%)",
        };

  const save = async (comment: string, color: AnnotationColor) => {
    if (selector === null || saving) {
      return;
    }
    setSaving(true);
    setFailure(null);
    const bounds = commentPinned ? commentBounds.current : null;
    const saved = await onAnnotate(
      selector,
      comment,
      color,
      bounds ?? undefined,
    );
    setSaving(false);
    if (saved) {
      rangeRef.current = null;
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
      rangeRef.current = null;
      setSelector(null);
      setPosition(null);
      document.getSelection()?.removeAllRanges();
    } else {
      setFailure("保存失败，可重试");
    }
  };

  if (reanchorPending !== null && onReanchor !== undefined) {
    if (position === null) return null;
    return (
      <div
        className="bg-popover text-popover-foreground border-border fixed z-40 flex flex-col gap-1 rounded-lg border p-1 shadow-lg"
        style={toolbarStyle}
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
        label="新建标注"
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
          onColorChange={setColor}
          pending={saving}
          onCancel={cancelComment}
          onSave={(comment, color) => void save(comment, color)}
        />
        {failure !== null ? (
          <p className="text-destructive px-2 pb-1 text-xs" role="alert">
            {failure}
          </p>
        ) : null}
      </AnnotationFloatingCard>
    );
  }

  if (position === null) return null;

  function openAnnotation() {
    if (position === null) return;
    setColor(DEFAULT_ANNOTATION_COLOR);
    setCommentPinned(false);
    commentBounds.current = null;
    const below = position.bottom + 12;
    const above = position.top - annotationCardHeight - 12;
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
      style={toolbarStyle}
      role="toolbar"
      aria-label="标注选中文本"
    >
      <Button variant="ghost" size="xs" onClick={openAnnotation}>
        <Highlighter aria-hidden="true" />
        标注
      </Button>
    </div>
  );
}
