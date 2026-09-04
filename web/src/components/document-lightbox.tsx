// Full-viewport viewer for the Markdown body's visual items — enhanced images
// and rendered Mermaid diagrams.
//
// Opened through the article's event delegation (see PreviewContent), it owns
// only data snapshots of the body items — never body DOM references — so a
// document hot swap can simply drop it. The dialog is modal on purpose: Base
// UI locks the page scroll while open, which is exactly the reading-position
// contract this feature must keep (open, navigate, zoom, rotate, close — the
// window's scrollY and hash never move).
//
// Lifecycle: the parent keeps the snapshot state and the open flag apart.
// Closing requests only flip `open` to false; the popup stays mounted while
// Base UI runs its exit transition (data-ending-style), and the parent drops
// the snapshot once `onClosed` reports the transition finished — so the exit
// animation is a first-class part of the component, not a race between a CSS
// duration and a JS timer.
//
// Zoom is expressed relative to the fitted baseline, not the natural pixels:
// zoom 1 means "fit the current rotation into the viewport". The base fit is
// recomputed per rotation because a 90° turn swaps the visual width/height, so
// a wide landscape image rotated upright still fits instead of overflowing.

import { Dialog } from "@base-ui/react/dialog";
import {
  ChevronLeft,
  ChevronRight,
  RotateCcw,
  RotateCw,
  X,
  ZoomIn,
  ZoomOut,
} from "lucide-react";
import {
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import type { LightboxItem } from "../lib/document-lightbox";

// zoom 1 is the fitted size; each step scales by 1.25 up to 5x. Dividing and
// multiplying by the same factor makes the zoom-out path retrace the zoom-in
// path exactly, so the sequence is stable for tests and muscle memory alike.
const MIN_ZOOM = 1;
const MAX_ZOOM = 5;
const ZOOM_STEP = 1.25;
// Pixel-mode trackpads report small deltas while a physical mouse notch is
// commonly around 100px. Exponential scaling preserves equal-feeling in/out
// motion; clamping each event to 100px caps a single notch at about 8.3%.
const WHEEL_ZOOM_SENSITIVITY = 0.0008;
const MAX_WHEEL_DELTA_PX = 100;
const WHEEL_LINE_HEIGHT_PX = 16;

type Rotation = 0 | 90 | 180 | 270;

interface Size {
  width: number;
  height: number;
}

interface PanDragState {
  pointerId: number;
  startX: number;
  startY: number;
  originX: number;
  originY: number;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function zoomInOneStep(value: number): number {
  return Math.min(MAX_ZOOM, value * ZOOM_STEP);
}

function zoomOutOneStep(value: number): number {
  return Math.max(MIN_ZOOM, value / ZOOM_STEP);
}

function normalizedWheelDelta(event: WheelEvent): number {
  let delta = event.deltaY;
  if (event.deltaMode === WheelEvent.DOM_DELTA_LINE) {
    delta *= WHEEL_LINE_HEIGHT_PX;
  } else if (event.deltaMode === WheelEvent.DOM_DELTA_PAGE) {
    delta *= window.innerHeight;
  }
  return clamp(delta, -MAX_WHEEL_DELTA_PX, MAX_WHEEL_DELTA_PX);
}

// True when the pointer landed on selectable vector text — a `<text>`/`<tspan>`
// or a `<foreignObject>` label (Mermaid renders many labels as HTML inside a
// foreignObject). A mouse press there belongs to native text selection, never
// to the pan.
function isVectorTextTarget(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) {
    return false;
  }
  return (
    target.closest("text, tspan") !== null ||
    target.closest("foreignObject") !== null
  );
}

export interface DocumentLightboxProps {
  items: LightboxItem[];
  index: number;
  open: boolean;
  onIndexChange(index: number): void;
  onClose(): void;
  onClosed(): void;
}

export function DocumentLightbox({
  items,
  index,
  open,
  onIndexChange,
  onClose,
  onClosed,
}: DocumentLightboxProps) {
  const item = items[index];

  // Base UI applies data-starting-style only when `open` flips false→true on
  // an already-mounted Root — a Root that first-mounts with open=true renders
  // its popup straight into the final state and skips the enter transition.
  // The parent mounts this component and flips `open` in the same commit, so
  // the Root is primed one commit behind: it mounts closed and the first
  // effect flips the internal flag, making the popup enter through its
  // starting style exactly like every later open of the same Root.
  const [primed, setPrimed] = useState(false);
  useEffect(() => {
    setPrimed(true);
  }, []);

  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [rotation, setRotation] = useState<Rotation>(0);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [panning, setPanning] = useState(false);
  // Measured geometry. Stays at zero where no layout engine runs (jsdom) or
  // before the ResizeObserver reports; the transform math then falls back to
  // the neutral fit and skips pan clamping rather than collapsing it.
  const [stage, setStage] = useState<Size>({ width: 0, height: 0 });
  const [imageLayout, setImageLayout] = useState<Size>({
    width: 0,
    height: 0,
  });

  // The dialog's portaled content mounts on a later commit than the component
  // itself, so the geometry observer must attach through callback refs: an
  // effect with [] deps runs once while the nodes are still null and never
  // again, which would silently leave the fit and the pan clamp without
  // measurements in a real browser.
  const stageNodeRef = useRef<HTMLDivElement | null>(null);
  const imageNodeRef = useRef<HTMLImageElement | null>(null);
  const visualNodeRef = useRef<HTMLElement | null>(null);
  const observerRef = useRef<ResizeObserver | null>(null);
  const dragRef = useRef<PanDragState | null>(null);

  const handleImageWheel = useCallback((event: WheelEvent) => {
    if (event.deltaX === 0 && event.deltaY === 0) {
      return;
    }
    // React delegates wheel events through a passive root listener, where
    // preventDefault cannot stop native scrolling. This handler is attached
    // directly to the visual item with passive:false below so the wheel belongs
    // to the preview while the pointer is over it. Vertical wheel zooms;
    // horizontal wheel only belongs to the Lightbox (the zoomed visual can
    // overflow the stage, and an unclaimed horizontal gesture would scroll or
    // trigger history navigation), so it is claimed but never reinterpreted as
    // zoom. Scale continuously from the normalized delta: trackpad gestures
    // stay fine-grained, large mouse-wheel deltas cannot jump straight through
    // the 1–5x range, and inverse deltas retrace the same multiplicative path.
    event.preventDefault();
    if (event.deltaY === 0) {
      return;
    }
    const scaleFactor = Math.exp(
      -normalizedWheelDelta(event) * WHEEL_ZOOM_SENSITIVITY,
    );
    setZoom((value) => clamp(value * scaleFactor, MIN_ZOOM, MAX_ZOOM));
  }, []);

  const ensureObserver = useCallback(() => {
    if (observerRef.current === null) {
      observerRef.current = new ResizeObserver((entries) => {
        for (const entry of entries) {
          const size = {
            width: entry.contentRect.width,
            height: entry.contentRect.height,
          };
          if (entry.target === stageNodeRef.current) {
            setStage(size);
          } else if (entry.target === imageNodeRef.current) {
            setImageLayout(size);
          }
        }
      });
    }
    return observerRef.current;
  }, []);

  const handleStageRef = useCallback(
    (node: HTMLDivElement | null) => {
      const observer = observerRef.current;
      if (stageNodeRef.current !== null && observer !== null) {
        observer.unobserve(stageNodeRef.current);
      }
      stageNodeRef.current = node;
      if (node !== null) {
        ensureObserver().observe(node);
      }
    },
    [ensureObserver],
  );

  const handleVisualRef = useCallback(
    (node: HTMLElement | null) => {
      visualNodeRef.current?.removeEventListener("wheel", handleImageWheel);
      visualNodeRef.current = node;
      if (node !== null) {
        node.addEventListener("wheel", handleImageWheel, { passive: false });
      }
    },
    [handleImageWheel],
  );

  const handleImageRef = useCallback(
    (node: HTMLImageElement | null) => {
      const observer = observerRef.current;
      const previousImage = imageNodeRef.current;
      if (previousImage !== null && observer !== null) {
        observer.unobserve(previousImage);
      }
      imageNodeRef.current = node;
      if (node !== null) {
        ensureObserver().observe(node);
        handleVisualRef(node);
      } else if (visualNodeRef.current === previousImage) {
        handleVisualRef(null);
      }
    },
    [ensureObserver, handleVisualRef],
  );

  useEffect(
    () => () => {
      observerRef.current?.disconnect();
      observerRef.current = null;
    },
    [],
  );

  // Every item starts from the fitted baseline: zoom, rotation, and pan are
  // per-item viewing state, not document-wide state. The reset runs as a
  // layout effect so it lands before the browser paints — the new item never
  // shows a frame carrying the previous item's transform — and an in-flight
  // drag dies with the switch (dragRef and the panning flag reset too) instead
  // of leaking pointer moves into the new item's pan.
  // biome-ignore lint/correctness/useExhaustiveDependencies: index is the deliberate re-run trigger — switching items must reset the viewing state even though the body never reads it.
  useLayoutEffect(() => {
    dragRef.current = null;
    setPanning(false);
    setZoom(MIN_ZOOM);
    setRotation(0);
    setPan({ x: 0, y: 0 });
    setImageLayout({ width: 0, height: 0 });
  }, [index]);

  const rotated = rotation === 90 || rotation === 270;
  const visualLayout =
    item?.kind === "image"
      ? imageLayout
      : {
          width: item?.intrinsicWidth ?? 0,
          height: item?.intrinsicHeight ?? 0,
        };
  const geometryKnown =
    stage.width > 0 &&
    stage.height > 0 &&
    visualLayout.width > 0 &&
    visualLayout.height > 0;
  // The stylesheet already contains-fits the unrotated layout (max-width /
  // max-height), so the unrotated base is 1; a 90° turn swaps the visual axes,
  // and the base shrinks so the rotated image fits again.
  const fitScale = geometryKnown
    ? Math.min(
        1,
        stage.width / (rotated ? visualLayout.height : visualLayout.width),
        stage.height / (rotated ? visualLayout.width : visualLayout.height),
      )
    : 1;
  const scale = fitScale * zoom;
  const renderedWidth = visualLayout.width * scale;
  const renderedHeight = visualLayout.height * scale;

  // How far the image may be dragged before its edges would leave the
  // viewport. With no measured geometry there is nothing to clamp against, so
  // the pan is left free instead of being pinned to zero.
  const maxPanX = geometryKnown
    ? Math.max(
        0,
        ((rotated ? renderedHeight : renderedWidth) - stage.width) / 2,
      )
    : Number.POSITIVE_INFINITY;
  const maxPanY = geometryKnown
    ? Math.max(
        0,
        ((rotated ? renderedWidth : renderedHeight) - stage.height) / 2,
      )
    : Number.POSITIVE_INFINITY;

  const clampPan = useCallback(
    (x: number, y: number) => ({
      x: clamp(x, -maxPanX, maxPanX),
      y: clamp(y, -maxPanY, maxPanY),
    }),
    [maxPanX, maxPanY],
  );

  // Shrinking the image or turning it back upright shrinks the draggable
  // range; re-clamp so the image never parks half off-screen.
  useEffect(() => {
    setPan((previous) => {
      const next = clampPan(previous.x, previous.y);
      if (next.x === previous.x && next.y === previous.y) {
        return previous;
      }
      return next;
    });
  }, [clampPan]);

  const hasPrevious = index > 0;
  const hasNext = index < items.length - 1;

  const goToPrevious = useCallback(() => {
    if (index > 0) {
      onIndexChange(index - 1);
    }
  }, [index, onIndexChange]);
  const goToNext = useCallback(() => {
    if (index < items.length - 1) {
      onIndexChange(index + 1);
    }
  }, [index, items.length, onIndexChange]);

  const zoomIn = () => {
    setZoom(zoomInOneStep);
  };
  const zoomOut = () => {
    setZoom(zoomOutOneStep);
  };
  const rotateClockwise = () => {
    setRotation((value) => ((value + 90) % 360) as Rotation);
  };
  const rotateCounterClockwise = () => {
    setRotation((value) => ((value + 270) % 360) as Rotation);
  };

  const handleVisualPointerDown = (event: ReactPointerEvent<HTMLElement>) => {
    if (event.pointerType === "mouse" && event.button !== 0) {
      return;
    }
    // Gesture arbitration on vector visuals: a mouse press on selectable text
    // belongs to the browser's native selection and must not start a pan.
    // Touch has no selection affordance here, so it always pans.
    if (
      item?.kind !== "image" &&
      event.pointerType === "mouse" &&
      isVectorTextTarget(event.target)
    ) {
      return;
    }
    // Panning only makes sense when the visual actually overflows the stage;
    // at the fitted baseline there is nothing to drag, and claiming the
    // gesture would only suppress selection and focus for no gain.
    const canPan = maxPanX > 0 || maxPanY > 0;
    if (!canPan) {
      return;
    }
    // This press is genuinely a pan: suppress the selection it would start.
    event.preventDefault();
    // Pointer capture keeps the drag on the image even when the pointer leaves
    // it; where the API is unavailable (or rejects the id) the move/up pair
    // still works as long as the pointer stays over the image.
    try {
      event.currentTarget.setPointerCapture(event.pointerId);
    } catch {
      // Capture is an optimization, not a requirement.
    }
    dragRef.current = {
      pointerId: event.pointerId,
      startX: event.clientX,
      startY: event.clientY,
      originX: pan.x,
      originY: pan.y,
    };
    setPanning(true);
  };

  // The snapshot keeps embedded links visually intact and hittable (they may
  // carry selectable text), but the Lightbox is a viewer: navigation never
  // happens. Capturing the click at the wrapper blocks it before it reaches
  // the anchor, whatever route the renderer used to synthesize it.
  const handleVectorClickCapture = (event: ReactMouseEvent<HTMLElement>) => {
    const target = event.target;
    if (target instanceof Element && target.closest("a") !== null) {
      event.preventDefault();
      event.stopPropagation();
    }
  };

  const handleVisualPointerMove = (event: ReactPointerEvent<HTMLElement>) => {
    const drag = dragRef.current;
    if (drag === null || drag.pointerId !== event.pointerId) {
      return;
    }
    const next = clampPan(
      drag.originX + event.clientX - drag.startX,
      drag.originY + event.clientY - drag.startY,
    );
    setPan(next);
  };

  const handleVisualPointerEnd = (event: ReactPointerEvent<HTMLElement>) => {
    if (dragRef.current?.pointerId !== event.pointerId) {
      return;
    }
    dragRef.current = null;
    setPanning(false);
  };

  // The popup fills the viewport and the stage is pointer-transparent, so a
  // press whose target is the popup itself landed on true empty space (not the
  // image, toolbar, or close button).
  const handlePopupPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget) {
      onClose();
    }
  };

  // Arrow keys switch items; panning stays a pointer gesture so one input
  // never carries two meanings. Escape is handled by the Dialog itself.
  const handleKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (event.key === "ArrowLeft") {
      if (hasPrevious) {
        event.preventDefault();
        goToPrevious();
      }
      return;
    }
    if (event.key === "ArrowRight" && hasNext) {
      event.preventDefault();
      goToNext();
    }
  };

  if (item === undefined) {
    return null;
  }

  return (
    // Every closing entrance — Dialog.Close, Escape, a blank-area press —
    // funnels through onOpenChange(false); the popup then stays mounted for
    // its exit transition, and onOpenChangeComplete(false) is the one signal
    // the parent waits for before dropping the snapshot state.
    <Dialog.Root
      open={open && primed}
      onOpenChange={(next) => {
        if (!next) {
          onClose();
        }
      }}
      onOpenChangeComplete={(next) => {
        if (!next) {
          onClosed();
        }
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="image-lightbox-backdrop" />
        <Dialog.Popup
          className="image-lightbox"
          onPointerDown={handlePopupPointerDown}
          onKeyDown={handleKeyDown}
        >
          <Dialog.Title className="sr-only">视觉内容预览</Dialog.Title>
          <Dialog.Close
            className="image-lightbox-close"
            aria-label="关闭视觉内容预览"
          >
            <X aria-hidden="true" />
          </Dialog.Close>
          {/* The stage is the one coordinate system for layout, rotation fit,
           * and pan clamping: the stylesheet reserves the toolbar zone here,
           * and the transform math measures this same box. It is
           * pointer-transparent so blank-area presses still reach the popup.
           * The enter/exit fade and scale ride on the stage (never on the
           * image below, whose transform carries live zoom/rotate/pan and
           * must never transition). */}
          <div
            className="image-lightbox-stage"
            data-visual-kind={item.kind}
            ref={handleStageRef}
          >
            {item.kind === "image" ? (
              <img
                ref={handleImageRef}
                className="image-lightbox-image"
                src={item.src}
                srcSet={item.srcSet ?? undefined}
                sizes={item.sizes ?? undefined}
                alt={item.alt}
                title={item.title ?? undefined}
                draggable={false}
                style={{
                  transform: `translate3d(${pan.x}px, ${pan.y}px, 0) rotate(${rotation}deg) scale(${scale})`,
                  cursor: panning ? "grabbing" : undefined,
                }}
                data-panning={panning ? "true" : undefined}
                onPointerDown={handleVisualPointerDown}
                onPointerMove={handleVisualPointerMove}
                onPointerUp={handleVisualPointerEnd}
                onPointerCancel={handleVisualPointerEnd}
              />
            ) : item.markup.trimStart().startsWith("<svg") ? (
              <div
                ref={handleVisualRef}
                className="image-lightbox-vector-transform"
                role="img"
                aria-label={item.alt}
                title={item.title ?? undefined}
                style={{
                  transform: `translate3d(${pan.x}px, ${pan.y}px, 0) rotate(${rotation}deg)`,
                  cursor: panning ? "grabbing" : undefined,
                }}
                data-panning={panning ? "true" : undefined}
                onPointerDown={handleVisualPointerDown}
                onPointerMove={handleVisualPointerMove}
                onPointerUp={handleVisualPointerEnd}
                onPointerCancel={handleVisualPointerEnd}
                onClickCapture={handleVectorClickCapture}
              >
                <div
                  className="image-lightbox-vector"
                  style={{
                    width: `${renderedWidth}px`,
                    height: `${renderedHeight}px`,
                  }}
                  // biome-ignore lint/security/noDangerouslySetInnerHtml: markup is serialized from the rendered SVG selected from the current document DOM.
                  dangerouslySetInnerHTML={{ __html: item.markup }}
                />
              </div>
            ) : null}
          </div>
          <div className="image-lightbox-toolbar">
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="上一项"
              disabled={!hasPrevious}
              onClick={goToPrevious}
            >
              <ChevronLeft aria-hidden="true" />
            </button>
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="缩小图片"
              disabled={zoom <= MIN_ZOOM}
              onClick={zoomOut}
            >
              <ZoomOut aria-hidden="true" />
            </button>
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="放大图片"
              disabled={zoom >= MAX_ZOOM}
              onClick={zoomIn}
            >
              <ZoomIn aria-hidden="true" />
            </button>
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="逆时针旋转"
              onClick={rotateCounterClockwise}
            >
              <RotateCcw aria-hidden="true" />
            </button>
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="顺时针旋转"
              onClick={rotateClockwise}
            >
              <RotateCw aria-hidden="true" />
            </button>
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="下一项"
              disabled={!hasNext}
              onClick={goToNext}
            >
              <ChevronRight aria-hidden="true" />
            </button>
            <span className="image-lightbox-counter">
              <span aria-hidden="true">
                {index + 1} / {items.length}
              </span>
              <span className="sr-only">
                第 {index + 1} 项，共 {items.length} 项
              </span>
            </span>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
