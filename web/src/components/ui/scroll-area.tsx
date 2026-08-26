import { ScrollArea as ScrollAreaPrimitive } from "@base-ui/react/scroll-area";
import { useEffect, useRef } from "react";

import { cn } from "@/lib/utils";

// Content is part of Base UI's expected ScrollArea anatomy: it observes its
// own box with a ResizeObserver and re-runs computeThumbPosition() when the
// content resizes without the viewport box changing (e.g. a tree collapsing
// inside it). Skipping it leaves those content-driven resizes unmeasured.
// contentProps lets callers override its default `minWidth: fit-content` —
// which a vertical-only scroll area must do to keep the x axis from growing.
interface ScrollAreaProps extends ScrollAreaPrimitive.Root.Props {
  contentProps?: ScrollAreaPrimitive.Content.Props;
  // "scrolling" hides the scrollbar until the viewport actually scrolls (and
  // for a short grace period after it stops): the sidebar and the TOC rail
  // otherwise reserve a permanent visual gutter next to their content. The
  // scrollbar DOM always stays mounted — only its opacity changes — so Base
  // UI's thumb geometry, the ResizeObserver and the sidebar's scroll
  // normalization never see a mount/unmount cycle.
  scrollbarVisibility?: "always" | "scrolling";
}

// How long the scrollbar stays up after the last scroll event.
const SCROLLING_GRACE_MS = 700;

function ScrollArea({
  className,
  children,
  contentProps,
  scrollbarVisibility = "always",
  ...props
}: ScrollAreaProps) {
  const rootRef = useRef<HTMLDivElement>(null);
  const timerRef = useRef(0);
  const transient = scrollbarVisibility === "scrolling";

  // Scroll is high-frequency: the flag is written straight to the root's
  // dataset so no React state — and no re-render — ever happens per event.
  // A new event resets the timer; 700ms of quiet removes the flag and the
  // CSS fades the scrollbar back out.
  const handleScroll = transient
    ? () => {
        const root = rootRef.current;
        if (root === null) {
          return;
        }
        root.dataset.scrolling = "true";
        window.clearTimeout(timerRef.current);
        timerRef.current = window.setTimeout(() => {
          delete root.dataset.scrolling;
        }, SCROLLING_GRACE_MS);
      }
    : undefined;

  useEffect(() => {
    if (!transient) {
      return;
    }
    // Clearing the pending timer on unmount keeps the timeout from firing
    // on a detached root (harmless, but noisy in tests).
    return () => {
      window.clearTimeout(timerRef.current);
    };
  }, [transient]);

  return (
    <ScrollAreaPrimitive.Root
      ref={rootRef}
      data-slot="scroll-area"
      data-scrollbar-visibility={scrollbarVisibility}
      className={cn("relative", className)}
      {...props}
    >
      <ScrollAreaPrimitive.Viewport
        data-slot="scroll-area-viewport"
        className="size-full rounded-[inherit] transition-[color,box-shadow] outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50 focus-visible:outline-1"
        onScroll={handleScroll}
      >
        <ScrollAreaPrimitive.Content
          data-slot="scroll-area-content"
          {...contentProps}
        >
          {children}
        </ScrollAreaPrimitive.Content>
      </ScrollAreaPrimitive.Viewport>
      <ScrollBar />
      <ScrollAreaPrimitive.Corner />
    </ScrollAreaPrimitive.Root>
  );
}

function ScrollBar({
  className,
  orientation = "vertical",
  ...props
}: ScrollAreaPrimitive.Scrollbar.Props) {
  return (
    <ScrollAreaPrimitive.Scrollbar
      data-slot="scroll-area-scrollbar"
      data-orientation={orientation}
      orientation={orientation}
      className={cn(
        "flex touch-none p-px transition-colors select-none data-horizontal:h-2.5 data-horizontal:flex-col data-horizontal:border-t data-horizontal:border-t-transparent data-vertical:h-full data-vertical:w-2.5 data-vertical:border-l data-vertical:border-l-transparent",
        className,
      )}
      {...props}
    >
      <ScrollAreaPrimitive.Thumb
        data-slot="scroll-area-thumb"
        className="relative flex-1 rounded-full bg-border"
      />
    </ScrollAreaPrimitive.Scrollbar>
  );
}

export { ScrollArea, ScrollBar };
