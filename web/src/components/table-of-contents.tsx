import { TableOfContents } from "lucide-react";
import { useState } from "react";

import type { TocItem } from "../api";
import { Button } from "./ui/button";
import { ScrollArea } from "./ui/scroll-area";
import { Sheet, SheetContent, SheetHeader, SheetTitle } from "./ui/sheet";
import { Tooltip, TooltipContent, TooltipTrigger } from "./ui/tooltip";

// TOCToggle is the toolbar button that switches the table of contents on and
// off. The pressed state is backed by URL state (preview.toc); when the current
// document has no H2-H4 headings the button disables itself.
export interface TOCToggleProps {
  enabled: boolean;
  available: boolean;
  onChange: (toc: boolean) => void;
}

export function TOCToggle({ enabled, available, onChange }: TOCToggleProps) {
  const label = !available
    ? "当前文档没有目录"
    : enabled
      ? "隐藏文档目录"
      : "显示文档目录";
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            className="reader-toc-toggle"
            aria-pressed={enabled}
            aria-label={label}
            disabled={!available}
            onClick={() => onChange(!enabled)}
          >
            <TableOfContents />
          </Button>
        }
      />
      <TooltipContent side="bottom">{label}</TooltipContent>
    </Tooltip>
  );
}

export interface TableOfContentsPanelProps {
  items: TocItem[];
  activeID: string | null;
  onNavigate(id: string): void;
}

interface TOCLinksProps {
  items: TocItem[];
  activeID: string | null;
  onNavigate(id: string): void;
}

// The link list shared by the desktop rail and the narrow-screen sheet. A
// native anchor jump would bypass the shared heading navigator (toolbar-aware
// scroll + URL funnel), so every link hands the interaction to onNavigate and
// suppresses the default jump.
function TOCLinks({ items, activeID, onNavigate }: TOCLinksProps) {
  return (
    <>
      {items.map((item) => {
        const active = item.id === activeID;
        return (
          <a
            key={item.id}
            href={`#${encodeURIComponent(item.id)}`}
            className="reader-toc-link"
            data-level={item.level}
            data-active={active ? "true" : "false"}
            aria-current={active ? "location" : undefined}
            onClick={(event) => {
              event.preventDefault();
              onNavigate(item.id);
            }}
          >
            {item.text}
          </a>
        );
      })}
    </>
  );
}

export function TableOfContentsPanel({
  items,
  activeID,
  onNavigate,
}: TableOfContentsPanelProps) {
  return (
    <nav className="reader-toc" aria-label="文档目录">
      {/* The same transient-scrollbar ScrollArea the sidebar uses, so both
       * scrollable rails share one scrollbar behavior. */}
      <ScrollArea className="reader-toc-scroll" scrollbarVisibility="scrolling">
        <div className="reader-toc-content">
          <p className="reader-toc-title">本页目录</p>
          <TOCLinks items={items} activeID={activeID} onNavigate={onNavigate} />
        </div>
      </ScrollArea>
    </nav>
  );
}

// Narrow screens (< 1200px) replace the desktop rail with this toolbar-triggered
// sheet. It is a transient navigation UI and deliberately never touches
// preview.toc, so reloading a default toc=true URL never pops the sheet open.
export function TableOfContentsSheet({
  items,
  activeID,
  onNavigate,
}: TableOfContentsPanelProps) {
  const [open, setOpen] = useState(false);

  const navigate = (id: string) => {
    setOpen(false);
    // Close the dialog first and hand the body scroll to the next frame, so
    // the dialog's scroll lock and the window scroll never collide.
    requestAnimationFrame(() => {
      onNavigate(id);
    });
  };

  return (
    <>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              variant="ghost"
              size="icon-sm"
              className="reader-toc-sheet-trigger"
              aria-label="打开文档目录"
              onClick={() => setOpen(true)}
            >
              <TableOfContents />
            </Button>
          }
        />
        <TooltipContent side="bottom">打开文档目录</TooltipContent>
      </Tooltip>

      <Sheet open={open} onOpenChange={setOpen}>
        <SheetContent side="right" className="reader-toc-sheet">
          <SheetHeader>
            <SheetTitle>本页目录</SheetTitle>
          </SheetHeader>
          {/* Same ScrollArea behavior as the desktop rail, so the narrow
           * screen's outline scrolls exactly like the wide one. */}
          <ScrollArea
            className="reader-toc-sheet-scroll"
            scrollbarVisibility="scrolling"
            role="navigation"
            aria-label="文档目录"
          >
            <TOCLinks items={items} activeID={activeID} onNavigate={navigate} />
          </ScrollArea>
        </SheetContent>
      </Sheet>
    </>
  );
}
