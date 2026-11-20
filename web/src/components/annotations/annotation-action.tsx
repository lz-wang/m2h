import type { ComponentProps } from "react";
import { Button } from "@/components/ui/button";
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip";
import { cn } from "@/lib/utils";

export function AnnotationAction({
  label,
  tooltip = label,
  children,
  className,
  ...props
}: ComponentProps<typeof Button> & { label: string; tooltip?: string }) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant="ghost"
            size="icon-xs"
            aria-label={label}
            focusableWhenDisabled
            className={cn("aria-disabled:opacity-50", className)}
            {...props}
          />
        }
      >
        {children}
      </TooltipTrigger>
      <TooltipContent role="tooltip">{tooltip}</TooltipContent>
    </Tooltip>
  );
}
