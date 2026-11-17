import { describe, expect, it } from "vitest";

import {
  exportAnnotationsJSON,
  exportAnnotationsMarkdown,
  exportSingleAnnotationMarkdown,
  type ExportAnnotation,
  type ExportDocument,
} from "./annotation-export";

const document: ExportDocument = {
  title: "设计文档",
  path: "design.md",
  ref: { rootId: "root-1", path: "design.md" },
  hash: "sha256:abc",
};

function annotation(overrides: Partial<ExportAnnotation> = {}): ExportAnnotation {
  return {
    id: "a1",
    documentPath: "design.md",
    documentHash: "sha256:abc",
    authorName: "alice",
    comment: "需要补充备份方案",
    selector: {
      quote: {
        exact: "使用 SQLite",
        prefix: "建议",
        suffix: "作为存储",
      },
      position: { start: 2, end: 9 },
      headingId: "存储",
      version: 1,
    },
    createdAt: 1760000000000,
    ...overrides,
  };
}

describe("exportAnnotationsMarkdown", () => {
  it("renders quotes, comments, authors and sections", () => {
    const markdown = exportAnnotationsMarkdown(document, [annotation()]);
    expect(markdown).toContain("# 设计文档 — 标注");
    expect(markdown).toContain("文档路径：design.md");
    expect(markdown).toContain("## 1. 存储");
    expect(markdown).toContain("> 使用 SQLite");
    expect(markdown).toContain("需要补充备份方案");
    expect(markdown).toContain("— alice ·");
  });

  it("marks highlight-only entries and anchor states", () => {
    const markdown = exportAnnotationsMarkdown(document, [
      annotation({ comment: "", status: "orphaned", id: "a2" }),
    ]);
    expect(markdown).toContain("仅高亮，无评论");
    expect(markdown).toContain("未定位");
  });

  it("states the empty set plainly", () => {
    expect(exportAnnotationsMarkdown(document, [])).toContain("暂无标注");
  });
});

describe("exportAnnotationsJSON", () => {
  it("keeps the complete selector shape and parses back", () => {
    const raw = exportAnnotationsJSON(document, [annotation()]);
    interface ParsedExport {
      document: { documentRef: { rootId: string } };
      annotations: Array<{ selector: { position: { start: number } } }>;
    }
    const parsed: ParsedExport = JSON.parse(raw);
    expect(parsed.document.documentRef.rootId).toBe("root-1");
    expect(parsed.annotations[0]?.selector.position.start).toBe(2);
  });
});

describe("exportSingleAnnotationMarkdown", () => {
  it("renders one annotation with document context", () => {
    const markdown = exportSingleAnnotationMarkdown(document, annotation());
    expect(markdown).toContain("# 设计文档 的标注");
    expect(markdown).toContain("所在章节：存储");
    expect(markdown).toContain("> 使用 SQLite");
  });
});
