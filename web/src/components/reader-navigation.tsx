import { ArrowDownToLine, ArrowUpToLine } from "lucide-react";
import { useEffect, useState } from "react";

import { Button } from "./ui/button";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";

// Whether the reading viewport currently sits at the document's top/bottom
// edge. The reader scrolls the window itself (see index.css), so this watches
// window scroll and resize events — rAF-throttled, at most one state write
// per frame — never a setState per scroll event. A ResizeObserver on the
// document element covers the reader's late-arriving content: a body that
// grows after mount (client-rendered article, late images) fires neither a
// scroll nor a resize, but it does resize the document.
export function useScrollBoundary(): { atTop: boolean; atBottom: boolean } {
  const [boundary, setBoundary] = useState(measureScrollBoundary);

  useEffect(() => {
    let frame = 0;
    const update = () => {
      frame = 0;
      setBoundary(measureScrollBoundary());
    };
    const schedule = () => {
      if (frame !== 0) {
        return;
      }
      frame = requestAnimationFrame(update);
    };
    window.addEventListener("scroll", schedule, { passive: true });
    window.addEventListener("resize", schedule);
    const observer = new ResizeObserver(schedule);
    observer.observe(document.documentElement);
    return () => {
      if (frame !== 0) {
        cancelAnimationFrame(frame);
      }
      window.removeEventListener("scroll", schedule);
      window.removeEventListener("resize", schedule);
      observer.disconnect();
    };
  }, []);

  return boundary;
}

function measureScrollBoundary(): { atTop: boolean; atBottom: boolean } {
  return {
    atTop: window.scrollY <= 1,
    atBottom:
      window.scrollY + window.innerHeight >=
      document.documentElement.scrollHeight - 1,
  };
}

// Floating reader navigation: a fixed vertical button pair in the reader's
// bottom-right corner that jumps the window (the reader's scroller) to the
// document edges. The controls report their state rather than hiding on a
// timer: at an edge the matching jump simply disables, keeping the affordance
// stable for a document reader. The group rides above normal content but
// below overlays (Sheet/Menu/Tooltip) and clears the home-indicator safe
// area on notched devices.
//
// Positioning stays `fixed` (the whole reader scrolls the window — heading
// spy, hash following and scroll restore all rely on that): when the TOC rail
// is visible, the CSS shifts the group left by the rail's width so the
// buttons sit beside the reader canvas instead of over the rail.
export function ReaderNavigation({ tocVisible }: { tocVisible: boolean }) {
  const { atTop, atBottom } = useScrollBoundary();

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  const scrollToBottom = () => {
    window.scrollTo({
      top: document.documentElement.scrollHeight,
      behavior: "smooth",
    });
  };

  return (
    <nav
      className="reader-navigation"
      data-toc-visible={tocVisible ? "true" : "false"}
      aria-label="阅读位置导航"
    >
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="回到顶部"
              disabled={atTop}
              onClick={scrollToTop}
            >
              <ArrowUpToLine aria-hidden="true" />
            </Button>
          }
        />
        <TooltipContent side="left">回到顶部</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              aria-label="前往底部"
              disabled={atBottom}
              onClick={scrollToBottom}
            >
              <ArrowDownToLine aria-hidden="true" />
            </Button>
          }
        />
        <TooltipContent side="left">前往底部</TooltipContent>
      </Tooltip>
    </nav>
  );
}
