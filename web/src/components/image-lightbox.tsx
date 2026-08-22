// Full-viewport image viewer for the Markdown body's enhanced images.
//
// Opened through the article's event delegation (see PreviewContent), it owns
// only data snapshots of the body images — never body DOM references — so a
// document hot swap can simply drop it. The dialog is modal on purpose: Base UI
// locks the page scroll while open, which is exactly the reading-position
// contract this feature must keep (open, navigate, zoom, rotate, close — the
// window's scrollY and hash never move).
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
  type PointerEvent as ReactPointerEvent,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";

import type { LightboxImage } from "../lib/image-lightbox";

// zoom 1 is the fitted size; each step scales by 1.25 up to 5x. Dividing and
// multiplying by the same factor makes the zoom-out path retrace the zoom-in
// path exactly, so the sequence is stable for tests and muscle memory alike.
const MIN_ZOOM = 1;
const MAX_ZOOM = 5;
const ZOOM_STEP = 1.25;

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

export interface ImageLightboxProps {
  images: LightboxImage[];
  index: number;
  onIndexChange(index: number): void;
  onClose(): void;
}

export function ImageLightbox({
  images,
  index,
  onIndexChange,
  onClose,
}: ImageLightboxProps) {
  const image = images[index];

  const [zoom, setZoom] = useState(MIN_ZOOM);
  const [rotation, setRotation] = useState<Rotation>(0);
  const [pan, setPan] = useState({ x: 0, y: 0 });
  const [panning, setPanning] = useState(false);
  // Measured geometry. Stays at zero where no layout engine runs (jsdom) or
  // before the ResizeObserver reports; the transform math then falls back to
  // the neutral fit and skips pan clamping rather than collapsing it.
  const [stage, setStage] = useState<Size>({ width: 0, height: 0 });
  const [layout, setLayout] = useState<Size>({ width: 0, height: 0 });

  const popupRef = useRef<HTMLDivElement>(null);
  const imageRef = useRef<HTMLImageElement>(null);
  const dragRef = useRef<PanDragState | null>(null);

  // Every image starts from the fitted baseline: zoom, rotation, and pan are
  // per-image viewing state, not document-wide state.
  // biome-ignore lint/correctness/useExhaustiveDependencies: index is the deliberate re-run trigger — switching images must reset the viewing state even though the body never reads it.
  useEffect(() => {
    setZoom(MIN_ZOOM);
    setRotation(0);
    setPan({ x: 0, y: 0 });
  }, [index]);

  // Track the popup (the panning viewport) and the image (its CSS-fitted
  // layout size, unaffected by the transform) so the fit and pan bounds can be
  // recomputed on resize, rotation, and image switches.
  useEffect(() => {
    const popup = popupRef.current;
    const imageElement = imageRef.current;
    if (popup === null || imageElement === null) {
      return;
    }
    const observer = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const size = {
          width: entry.contentRect.width,
          height: entry.contentRect.height,
        };
        if (entry.target === popup) {
          setStage(size);
        } else {
          setLayout(size);
        }
      }
    });
    observer.observe(popup);
    observer.observe(imageElement);
    return () => {
      observer.disconnect();
    };
  }, []);

  const rotated = rotation === 90 || rotation === 270;
  const geometryKnown =
    stage.width > 0 &&
    stage.height > 0 &&
    layout.width > 0 &&
    layout.height > 0;
  // The stylesheet already contains-fits the unrotated layout (max-width /
  // max-height), so the unrotated base is 1; a 90° turn swaps the visual axes,
  // and the base shrinks so the rotated image fits again.
  const fitScale = geometryKnown
    ? Math.min(
        1,
        stage.width / (rotated ? layout.height : layout.width),
        stage.height / (rotated ? layout.width : layout.height),
      )
    : 1;
  const scale = fitScale * zoom;

  // How far the image may be dragged before its edges would leave the
  // viewport. With no measured geometry there is nothing to clamp against, so
  // the pan is left free instead of being pinned to zero.
  const maxPanX = geometryKnown
    ? Math.max(
        0,
        ((rotated ? layout.height : layout.width) * scale - stage.width) / 2,
      )
    : Number.POSITIVE_INFINITY;
  const maxPanY = geometryKnown
    ? Math.max(
        0,
        ((rotated ? layout.width : layout.height) * scale - stage.height) / 2,
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
  const hasNext = index < images.length - 1;

  const goToPrevious = useCallback(() => {
    if (index > 0) {
      onIndexChange(index - 1);
    }
  }, [index, onIndexChange]);
  const goToNext = useCallback(() => {
    if (index < images.length - 1) {
      onIndexChange(index + 1);
    }
  }, [index, images.length, onIndexChange]);

  const zoomIn = () => {
    setZoom((value) => Math.min(MAX_ZOOM, value * ZOOM_STEP));
  };
  const zoomOut = () => {
    setZoom((value) => Math.max(MIN_ZOOM, value / ZOOM_STEP));
  };
  const rotateClockwise = () => {
    setRotation((value) => ((value + 90) % 360) as Rotation);
  };
  const rotateCounterClockwise = () => {
    setRotation((value) => ((value + 270) % 360) as Rotation);
  };

  const handleImagePointerDown = (
    event: ReactPointerEvent<HTMLImageElement>,
  ) => {
    if (event.pointerType === "mouse" && event.button !== 0) {
      return;
    }
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

  const handleImagePointerMove = (
    event: ReactPointerEvent<HTMLImageElement>,
  ) => {
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

  const handleImagePointerEnd = (
    event: ReactPointerEvent<HTMLImageElement>,
  ) => {
    if (dragRef.current?.pointerId !== event.pointerId) {
      return;
    }
    dragRef.current = null;
    setPanning(false);
  };

  // The popup fills the viewport, so a press whose target is the popup itself
  // landed on true empty space (not the image, toolbar, or close button).
  const handlePopupPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget) {
      onClose();
    }
  };

  // Arrow keys switch images; panning stays a pointer gesture so one input
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

  if (image === undefined) {
    return null;
  }

  return (
    <Dialog.Root
      open
      onOpenChange={(open) => {
        if (!open) {
          onClose();
        }
      }}
    >
      <Dialog.Portal>
        <Dialog.Backdrop className="image-lightbox-backdrop" />
        <Dialog.Popup
          ref={popupRef}
          className="image-lightbox"
          onPointerDown={handlePopupPointerDown}
          onKeyDown={handleKeyDown}
        >
          <Dialog.Title className="sr-only">图片预览</Dialog.Title>
          <Dialog.Close
            className="image-lightbox-close"
            aria-label="关闭图片预览"
          >
            <X aria-hidden="true" />
          </Dialog.Close>
          <img
            ref={imageRef}
            className="image-lightbox-image"
            src={image.src}
            srcSet={image.srcSet ?? undefined}
            sizes={image.sizes ?? undefined}
            alt={image.alt}
            title={image.title ?? undefined}
            draggable={false}
            style={{
              transform: `translate3d(${pan.x}px, ${pan.y}px, 0) rotate(${rotation}deg) scale(${scale})`,
              cursor: panning ? "grabbing" : undefined,
            }}
            data-panning={panning ? "true" : undefined}
            onPointerDown={handleImagePointerDown}
            onPointerMove={handleImagePointerMove}
            onPointerUp={handleImagePointerEnd}
            onPointerCancel={handleImagePointerEnd}
          />
          <div className="image-lightbox-toolbar">
            <button
              type="button"
              className="image-lightbox-control"
              aria-label="上一张图片"
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
              aria-label="下一张图片"
              disabled={!hasNext}
              onClick={goToNext}
            >
              <ChevronRight aria-hidden="true" />
            </button>
            <span className="image-lightbox-counter">
              <span aria-hidden="true">
                {index + 1} / {images.length}
              </span>
              <span className="sr-only">
                第 {index + 1} 张，共 {images.length} 张
              </span>
            </span>
          </div>
        </Dialog.Popup>
      </Dialog.Portal>
    </Dialog.Root>
  );
}
