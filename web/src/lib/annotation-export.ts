import type { AnnotationSelector } from "./annotation-selector";

export interface ExportAnnotation {
  id: string;
  documentPath: string;
  documentHash: string;
  authorName: string;
  comment: string;
  selector: AnnotationSelector;
  status?: "resolved" | "ambiguous" | "orphaned";
  createdAt: number;
  updatedAt: number;
}

export interface ExportDocument {
  title: string;
  path: string;
  ref: { rootId: string; path: string };
  hash: string;
}

export interface ReviewCredentials {
  url: string;
  token: string;
  expiresInMinutes: number;
}

// Format local time with its actual UTC offset for portable JSON consumers.
function localISO(date: Date): string {
  const offset = -date.getTimezoneOffset();
  const pad = (value: number) => String(Math.abs(value)).padStart(2, "0");
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}${offset >= 0 ? "+" : "-"}${pad(Math.trunc(offset / 60))}:${pad(offset % 60)}`;
}

export function exportAnnotationsMarkdown(
  document: ExportDocument,
  annotations: ExportAnnotation[],
  credentials: ReviewCredentials,
): string {
  const lines = [
    "# 文档评审意见",
    "",
    `- 文档: ${credentials.url}`,
    `- 版本: \`${document.hash}\``,
    `- 访问凭证: \`Authorization: Bearer ${credentials.token}\``,
    `- 凭证有效期: ${credentials.expiresInMinutes} 分钟（仅限此文档的当前版本）`,
    "",
  ];
  annotations.forEach((entry, index) => {
    const number = index + 1;
    lines.push(
      `<!--评审意见--${number}--START-->`,
      "",
      `### ${number}. ${entry.selector.headingId || "正文"}`,
      "",
      `- 正文引用: ${entry.selector.quote.exact.replaceAll("\n", "\n  ")}`,
      `- 正文定位: start ${entry.selector.position.start}, end ${entry.selector.position.end}`,
      `- 评审意见: ${(entry.comment || "（仅高亮，无评论）").replaceAll("\n", "\n  ")}`,
      `- 评审人员: ${entry.authorName}`,
      `- 评审时间: ${new Date(entry.updatedAt).toLocaleString("zh-CN", { hour12: false })}`,
      ...(entry.status && entry.status !== "resolved"
        ? [`- 定位状态: ${entry.status}`]
        : []),
      "",
      `<!--评审意见--${number}--END-->`,
      "",
    );
  });
  return lines.join("\n").trimEnd();
}

export function exportAnnotationsJSON(
  document: ExportDocument,
  annotations: ExportAnnotation[],
  credentials: ReviewCredentials,
): string {
  return JSON.stringify(
    {
      document: {
        title: document.title,
        path: document.path,
        url: credentials.url,
        authentication: {
          authorization: `Bearer ${credentials.token}`,
          expiresInMinutes: credentials.expiresInMinutes,
          scope: "current-document-version",
        },
        documentRef: document.ref,
        documentHash: document.hash,
      },
      exportedAt: localISO(new Date()),
      annotations: annotations.map((entry) => ({
        id: entry.id,
        documentPath: entry.documentPath,
        documentHash: entry.documentHash,
        authorName: entry.authorName,
        comment: entry.comment,
        status: entry.status ?? "resolved",
        selector: {
          ...entry.selector,
          headingId: entry.selector.headingId ?? "",
          tabId: entry.selector.tabId ?? "",
          blockHint: entry.selector.blockHint ?? "",
        },
        createdAt: localISO(new Date(entry.createdAt)),
        updatedAt: localISO(new Date(entry.updatedAt)),
      })),
    },
    null,
    2,
  );
}
