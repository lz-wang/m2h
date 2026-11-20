import { GripHorizontal, Pin, PinOff, X } from "lucide-react";
import {
  type KeyboardEvent,
  type PointerEvent,
  type ReactNode,
  useEffect,
  useRef,
  useState,
} from "react";
import { AnnotationAction } from "./annotation-action";

interface Bounds {
  left: number;
  top: number;
  width: number;
  height: number;
}

const resizeCorners = [
  { position: "top-left", label: "左上角", x: -1, y: -1 },
  { position: "top-right", label: "右上角", x: 1, y: -1 },
  { position: "bottom-left", label: "左下角", x: -1, y: 1 },
  { position: "bottom-right", label: "右下角", x: 1, y: 1 },
] as const;

type Adjustment = "move" | (typeof resizeCorners)[number];

export const annotationCardWidth = 290;
export const annotationCardHeight = 176;

const margin = 8;
const minWidth = 200;
const minHeight = 144;

function fitBounds(bounds: Bounds): Bounds {
  const availableWidth = window.innerWidth - 2 * margin;
  const availableHeight = window.innerHeight - 2 * margin;
  const width = Math.min(availableWidth, bounds.width);
  const height = Math.min(availableHeight, bounds.height);
  return {
    width,
    height,
    left: Math.max(
      margin,
      Math.min(bounds.left, availableWidth + margin - width),
    ),
    top: Math.max(
      margin,
      Math.min(bounds.top, availableHeight + margin - height),
    ),
  };
}

function adjust(start: Bounds, mode: Adjustment, dx: number, dy: number) {
  if (mode === "move") {
    return fitBounds({
      ...start,
      left: start.left + dx,
      top: start.top + dy,
    });
  }
  const bounds = fitBounds(start);
  const right = bounds.left + bounds.width;
  const bottom = bounds.top + bounds.height;
  const width = Math.min(
    Math.max(minWidth, bounds.width + dx * mode.x),
    mode.x < 0 ? right - margin : window.innerWidth - bounds.left - margin,
  );
  const height = Math.min(
    Math.max(minHeight, bounds.height + dy * mode.y),
    mode.y < 0 ? bottom - margin : window.innerHeight - bounds.top - margin,
  );
  return {
    left: mode.x < 0 ? right - width : bounds.left,
    top: mode.y < 0 ? bottom - height : bounds.top,
    width,
    height,
  };
}

export function AnnotationFloatingCard({
  left,
  top,
  label = "标注窗口",
  closeDisabled = false,
  pinned = false,
  children,
  onPinnedChange,
  onAdjust,
  onEnter,
  onLeave,
  onClose,
}: {
  left: number;
  top: number;
  label?: string;
  closeDisabled?: boolean;
  pinned?: boolean;
  children: ReactNode;
  onPinnedChange?(pinned: boolean): void;
  onAdjust?(): void;
  onEnter?(): void;
  onLeave?(): void;
  onClose(): void;
}) {
  const [bounds, setBounds] = useState(() =>
    fitBounds({
      left,
      top,
      width: annotationCardWidth,
      height: annotationCardHeight,
    }),
  );
  const gesture = useRef<{
    pointerId: number;
    x: number;
    y: number;
    bounds: Bounds;
  } | null>(null);

  useEffect(() => {
    const fitToViewport = () => setBounds(fitBounds);
    window.addEventListener("resize", fitToViewport);
    return () => window.removeEventListener("resize", fitToViewport);
  }, []);

  function adjustmentEvents(mode: Adjustment) {
    return {
      onPointerDown(event: PointerEvent<HTMLButtonElement>) {
        if (event.button !== 0 || gesture.current !== null) return;
        event.preventDefault();
        gesture.current = {
          pointerId: event.pointerId,
          x: event.clientX,
          y: event.clientY,
          bounds,
        };
        event.currentTarget.setPointerCapture(event.pointerId);
      },
      onPointerMove(event: PointerEvent<HTMLButtonElement>) {
        const start = gesture.current;
        if (start?.pointerId !== event.pointerId) return;
        const dx = event.clientX - start.x;
        const dy = event.clientY - start.y;
        if (dx !== 0 || dy !== 0) onAdjust?.();
        setBounds(adjust(start.bounds, mode, dx, dy));
      },
      onLostPointerCapture() {
        gesture.current = null;
      },
      onKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
        const steps: Record<string, [number, number]> = {
          ArrowLeft: [-16, 0],
          ArrowRight: [16, 0],
          ArrowUp: [0, -16],
          ArrowDown: [0, 16],
        };
        const step = steps[event.key];
        if (!step) return;
        event.preventDefault();
        onAdjust?.();
        setBounds((current) => adjust(current, mode, ...step));
      },
    };
  }

  return (
    <section
      className="annotation-hover-card"
      style={bounds}
      aria-label={label}
      onMouseEnter={onEnter}
      onMouseLeave={() => {
        if (gesture.current === null) onLeave?.();
      }}
    >
      <div className="annotation-hover-header">
        {onPinnedChange ? (
          <AnnotationAction
            label="固定悬浮框"
            tooltip={pinned ? "取消固定悬浮框" : "固定悬浮框"}
            aria-pressed={pinned}
            onClick={() => onPinnedChange(!pinned)}
          >
            {pinned ? (
              <PinOff aria-hidden="true" />
            ) : (
              <Pin aria-hidden="true" />
            )}
          </AnnotationAction>
        ) : null}
        <AnnotationAction
          className="annotation-hover-move"
          label="移动标注窗口"
          tooltip="拖动或按方向键移动标注窗口"
          {...adjustmentEvents("move")}
        >
          <GripHorizontal aria-hidden="true" />
        </AnnotationAction>
        <AnnotationAction
          label="关闭标注窗口"
          disabled={closeDisabled}
          onClick={onClose}
        >
          <X aria-hidden="true" />
        </AnnotationAction>
      </div>
      <div className="annotation-hover-body">{children}</div>
      {resizeCorners.map((corner) => (
        <button
          key={corner.position}
          type="button"
          className="annotation-hover-corner"
          data-corner={corner.position}
          aria-label={`从${corner.label}调整标注窗口大小`}
          tabIndex={corner.position === "bottom-right" ? 0 : -1}
          {...adjustmentEvents(corner)}
        />
      ))}
    </section>
  );
}
