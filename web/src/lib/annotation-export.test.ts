import { describe, expect, it } from "vitest";
import {
  type ExportAnnotation,
  type ExportDocument,
  exportAnnotationsJSON,
  exportAnnotationsMarkdown,
} from "./annotation-export";

const document: ExportDocument = {
  title: "m2h",
  path: "README.md",
  ref: { rootId: "root-1", path: "README.md" },
  hash: "sha256:abc",
};
const credentials = {
  url: "https://docs.example.com/docs/raw/README.md",
  token: "m2h_test",
  expiresInMinutes: 30,
};
const entry: ExportAnnotation = {
  color: "amber",
  id: "a1",
  documentPath: "README.md",
  documentHash: "sha256:abc",
  authorName: "alice",
  comment: "建议修改",
  selector: {
    quote: { exact: "引用内容", prefix: "前", suffix: "后" },
    position: { start: 4, end: 8 },
    headingId: "特性",
    version: 1,
  },
  createdAt: 1760000000000,
  updatedAt: new Date(2026, 10, 22, 0, 52, 11).getTime(),
};

describe("agent review export", () => {
  it("includes the scoped credentials and numbered review delimiters in Markdown", () => {
    const result = exportAnnotationsMarkdown(
      document,
      [entry, { ...entry, id: "a2", comment: "" }],
      credentials,
    );
    expect(result).toContain("# 文档评审意见");
    expect(result).toContain(
      "- 文档: https://docs.example.com/docs/raw/README.md",
    );
    expect(result).toContain("- 版本: `sha256:abc`");
    expect(result).toContain("Authorization: Bearer m2h_test");
    expect(result).toContain("<!--评审意见--1--START-->");
    expect(result).toContain("<!--评审意见--2--END-->");
    expect(result).toContain("- 正文定位: start 4, end 8");
    expect(result).toContain("- 评审意见:");
    expect(result).toContain("- 评审人员: alice");
    expect(result).toContain("- 评审时间: 2026-11-22 00:52:11");
    expect(result).toContain("（仅高亮，无评论）");
  });
  it("produces valid JSON with credentials, selector, and offset timestamps", () => {
    const parsed = JSON.parse(
      exportAnnotationsJSON(document, [entry], credentials),
    ) as { annotations: Array<{ createdAt: string; updatedAt: string }> };
    expect(parsed.annotations[0]).toMatchObject({ color: "amber" });
    expect(parsed).toMatchObject({
      document: {
        title: "m2h",
        url: credentials.url,
        documentHash: document.hash,
        authentication: {
          authorization: "Bearer m2h_test",
          expiresInMinutes: 30,
        },
        documentRef: document.ref,
      },
      annotations: [
        {
          id: "a1",
          selector: {
            position: { start: 4, end: 8 },
            tabId: "",
            blockHint: "",
          },
        },
      ],
    });
    const [annotation] = parsed.annotations;
    expect(annotation.createdAt).toMatch(
      /\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d[+-]\d\d:\d\d/,
    );
    expect(Date.parse(annotation.createdAt)).toBe(entry.createdAt);
    expect(Date.parse(annotation.updatedAt)).toBe(entry.updatedAt);
  });
});
