// Annotation export: one annotation or a whole document's set, as
// clipboard-ready Markdown or loss-less JSON. Comments and quotes travel
// as plain text; nothing here interprets or emits HTML.

import type { AnnotationSelector } from "./annotation-selector";

export interface ExportAnnotation {
  id: string;
  documentPath: string;
  documentHash: string;
  authorName: string;
  comment: string;
  selector: AnnotationSelector;
  // Anchor state the reader computed for this export, if known.
  status?: "resolved" | "ambiguous" | "orphaned";
  createdAt: number;
}

export interface ExportDocument {
  title: string;
  path: string;
  ref: { rootId: string; path: string };
  hash: string;
}

function formatTime(milliseconds: number): string {
  return new Date(milliseconds).toLocaleString();
}

function headingOf(annotation: ExportAnnotation): string {
  return annotation.selector.headingId ?? "";
}

// exportAnnotationsMarkdown renders a document's annotations as a review
// document: quote, comment, author, section and anchor state per entry.
export function exportAnnotationsMarkdown(
  document: ExportDocument,
  annotations: ExportAnnotation[],
): string {
  const lines: string[] = [
    `# ${document.title} — 标注`,
    "",
    `- 文档路径：${document.path}`,
    `- 文档版本：${document.hash}`,
    `- 标注数量：${annotations.length}`,
    "",
  ];
  if (annotations.length === 0) {
    lines.push("（暂无标注）");
    return lines.join("\n");
  }
  annotations.forEach((annotation, index) => {
    const section = headingOf(annotation);
    lines.push(`## ${index + 1}. ${section === "" ? "正文" : section}`);
    lines.push("");
    lines.push(`> ${annotation.selector.quote.exact.replaceAll("\n", "\n> ")}`);
    lines.push("");
    lines.push(
      annotation.comment === "" ? "_（仅高亮，无评论）_" : annotation.comment,
    );
    lines.push("");
    lines.push(
      `— ${annotation.authorName} · ${formatTime(annotation.createdAt)}` +
        (annotation.status !== undefined && annotation.status !== "resolved"
          ? ` · 状态：${annotation.status === "ambiguous" ? "位置不确定" : "未定位"}`
          : ""),
    );
    lines.push("");
  });
  return lines.join("\n").trimEnd();
}

// exportAnnotationsJSON keeps the complete stored shape — selectors,
// identity, hash, ids and timestamps — for re-import or archival tooling.
export function exportAnnotationsJSON(
  document: ExportDocument,
  annotations: ExportAnnotation[],
): string {
  return JSON.stringify(
    {
      document: {
        title: document.title,
        path: document.path,
        documentRef: document.ref,
        documentHash: document.hash,
      },
      exportedAt: new Date().toISOString(),
      annotations: annotations.map((annotation) => ({
        id: annotation.id,
        documentPath: annotation.documentPath,
        documentHash: annotation.documentHash,
        authorName: annotation.authorName,
        comment: annotation.comment,
        status: annotation.status ?? "resolved",
        selector: annotation.selector,
        createdAt: annotation.createdAt,
      })),
    },
    null,
    2,
  );
}

// exportSingleAnnotationMarkdown renders one annotation with its document
// context — the clipboard shape for sharing a single remark.
export function exportSingleAnnotationMarkdown(
  document: ExportDocument,
  annotation: ExportAnnotation,
): string {
  const section = headingOf(annotation);
  return [
    `# ${document.title} 的标注`,
    "",
    `- 文档路径：${document.path}`,
    section === "" ? "" : `- 所在章节：${section}`,
    "",
    `> ${annotation.selector.quote.exact.replaceAll("\n", "\n> ")}`,
    "",
    annotation.comment === "" ? "_（仅高亮，无评论）_" : annotation.comment,
    "",
    `— ${annotation.authorName} · ${formatTime(annotation.createdAt)}`,
  ].join("\n");
}
