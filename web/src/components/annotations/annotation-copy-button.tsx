import {
  Check,
  CircleAlert,
  FileJson,
  FileText,
  LoaderCircle,
} from "lucide-react";
import { useEffect, useRef, useState } from "react";
import { AnnotationAction } from "./annotation-action";

export type AnnotationCopyFormat = "markdown" | "json";

export function AnnotationCopyButton({
  format,
  label,
  disabled = false,
  onCopy,
}: {
  format: AnnotationCopyFormat;
  label: string;
  disabled?: boolean;
  onCopy(): Promise<boolean>;
}) {
  const [state, setState] = useState<"idle" | "pending" | "copied" | "error">(
    "idle",
  );
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);
  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );
  const Icon = format === "markdown" ? FileText : FileJson;
  return (
    <>
      <AnnotationAction
        label={label}
        tooltip={state === "error" ? "复制失败，请重试" : label}
        disabled={disabled || state === "pending"}
        onClick={() => {
          if (timer.current) clearTimeout(timer.current);
          setState("pending");
          void onCopy()
            .then((ok) => {
              setState(ok ? "copied" : "error");
              timer.current = setTimeout(() => setState("idle"), 1500);
            })
            .catch(() => setState("error"));
        }}
      >
        {state === "copied" ? (
          <Check aria-hidden="true" />
        ) : state === "pending" ? (
          <LoaderCircle aria-hidden="true" className="animate-spin" />
        ) : state === "error" ? (
          <CircleAlert aria-hidden="true" className="text-destructive" />
        ) : (
          <Icon aria-hidden="true" />
        )}
      </AnnotationAction>
      {state === "error" ? (
        <span className="sr-only" role="alert">
          复制失败，请重试
        </span>
      ) : null}
    </>
  );
}
