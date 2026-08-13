import { TableOfContents } from "lucide-react";
import type { MouseEvent } from "react";

import type { TocItem } from "../api";
import { Button } from "./ui/button";
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
  onSelectHeading?(id: string): void;
}

export function TableOfContentsPanel({
  items,
  activeID,
  onSelectHeading,
}: TableOfContentsPanelProps) {
  const handleSelect = (
    event: MouseEvent<HTMLAnchorElement>,
    item: TocItem,
  ) => {
    // The reader body scrolls inside a ScrollArea viewport, so native anchor
    // navigation would not land correctly; drive the scroll ourselves and hand
    // the hash update to the caller so it goes through the single URL funnel.
    event.preventDefault();
    const heading = document.getElementById(item.id);
    if (heading !== null) {
      const reduceMotion = window.matchMedia(
        "(prefers-reduced-motion: reduce)",
      ).matches;
      heading.scrollIntoView({
        block: "start",
        behavior: reduceMotion ? "auto" : "smooth",
      });
    }
    onSelectHeading?.(item.id);
  };

  return (
    <nav className="reader-toc" aria-label="文档目录">
      <div className="reader-toc-scroll">
        <div className="reader-toc-content">
          <p className="reader-toc-title">本页目录</p>
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
                onClick={(event) => handleSelect(event, item)}
              >
                {item.text}
              </a>
            );
          })}
        </div>
      </div>
    </nav>
  );
}
