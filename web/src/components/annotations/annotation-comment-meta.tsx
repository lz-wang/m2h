import type { AnnotationItem } from "@/api";

export function AnnotationCommentMeta({ item }: { item: AnnotationItem }) {
  const updated = new Date(item.updatedAt);
  return (
    <span className="text-muted-foreground flex flex-wrap items-baseline gap-x-1.5 text-xs">
      <span className="break-all">{item.authorName}</span>
      <span aria-hidden="true">·</span>
      <time dateTime={updated.toISOString()}>{updated.toLocaleString()}</time>
    </span>
  );
}
