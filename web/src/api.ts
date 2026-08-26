// model.ts imports only types from this module, so pulling its URL builder in
// here creates no runtime cycle — and keeps every /raw/ address shaped by one
// encoder shared with the sidebar and toolbar links.
import { markdownURL } from "./model";

export interface FileSummary {
  path: string;
  name: string;
  title: string;
}

// RootSummary groups one preview root's documents. Files carry root-relative
// paths; in a multi-root workspace the root id prefixes the addressable
// (virtual) document path, so identity stays unique across roots. kind tells
// whether absolutePath names the served file itself ("file") or the directory
// the root-relative paths join onto ("directory") — a file root must not have
// its only document's path appended again. absolutePath is the server machine's
// canonical local path for the input; it is reported only when the server
// listens on loopback, so the copy-path affordances treat it as optional.
// pathSeparator is that machine's separator — the browser may run elsewhere,
// so joining a native path must use the server-reported separator.
export interface RootSummary {
  id: string;
  name: string;
  kind: RootKind;
  absolutePath?: string;
  pathSeparator: string;
  files: FileSummary[];
}

export type RootKind = "file" | "directory";

export interface DocumentRef {
  root: string;
  path: string;
}

// PreviewKind reports what the server is previewing, so the WebUI can hide
// file navigation when there is nothing to navigate: one file, one directory,
// or a workspace of several roots.
export type PreviewKind = "single" | "directory" | "workspace";

export interface FileListResponse {
  kind: PreviewKind;
  roots: RootSummary[];
  defaultDocument: DocumentRef | null;
  version: string;
}

export interface FrontMatterEntry {
  key: string;
  value: string;
}

export interface FrontMatter {
  entries: FrontMatterEntry[];
  date?: string;
  tags?: string[];
}

export interface TocItem {
  level: number;
  id: string;
  text: string;
}

export interface DocumentResponse {
  path: string;
  title: string;
  html: string;
  frontmatter: FrontMatter | null;
  toc: TocItem[];
}

export interface PreviewAPI {
  listFiles(signal?: AbortSignal): Promise<FileListResponse>;
  getDocument(path: string, signal?: AbortSignal): Promise<DocumentResponse>;
  // Fetches the document's original Markdown source (frontmatter included)
  // from /raw/<virtual-path> on demand — sharing keeps it out of every
  // /api/document response until the reader actually asks for the full text.
  getMarkdown(path: string, signal?: AbortSignal): Promise<string>;
}

export class APIError extends Error {
  readonly status: number;

  constructor(status: number, message: string) {
    super(message);
    this.name = "APIError";
    this.status = status;
  }
}

async function requestJSON(
  url: string,
  signal?: AbortSignal,
): Promise<unknown> {
  const response = await fetch(url, {
    headers: { Accept: "application/json" },
    signal,
  });
  const payload: unknown = await response.json().catch(() => null);
  if (!response.ok) {
    const message =
      isRecord(payload) && typeof payload.error === "string"
        ? payload.error
        : response.statusText;
    throw new APIError(response.status, message || `HTTP ${response.status}`);
  }
  return payload;
}

// /raw/ answers errors as plain text (http.Error), so unlike requestJSON there
// is no JSON error body to mine — the status alone decides.
async function requestText(url: string, signal?: AbortSignal): Promise<string> {
  const response = await fetch(url, {
    headers: { Accept: "text/markdown" },
    signal,
  });
  if (!response.ok) {
    throw new APIError(
      response.status,
      response.statusText || `HTTP ${response.status}`,
    );
  }
  return response.text();
}

function parseFileList(payload: unknown): FileListResponse {
  if (
    !isRecord(payload) ||
    typeof payload.version !== "string" ||
    !Array.isArray(payload.roots)
  ) {
    throw new Error("文件列表响应格式无效");
  }
  const roots = payload.roots.map((value) => {
    if (
      !isRecord(value) ||
      typeof value.id !== "string" ||
      typeof value.name !== "string" ||
      !isRootKind(value.kind) ||
      typeof value.absolutePath !== "string" ||
      (value.pathSeparator !== "/" && value.pathSeparator !== "\\")
    ) {
      throw new Error("文件列表响应格式无效");
    }
    if (!Array.isArray(value.files)) {
      throw new Error("文件列表响应格式无效");
    }
    return {
      id: value.id,
      name: value.name,
      kind: value.kind,
      absolutePath: value.absolutePath,
      pathSeparator: value.pathSeparator,
      files: value.files.map(parseFileSummary),
    };
  });
  return {
    kind: parsePreviewKind(payload.kind),
    roots,
    defaultDocument: parseDocumentRef(payload.defaultDocument),
    version: payload.version,
  };
}

function parseFileSummary(value: unknown): FileSummary {
  if (
    !isRecord(value) ||
    typeof value.path !== "string" ||
    typeof value.name !== "string" ||
    typeof value.title !== "string"
  ) {
    throw new Error("文件条目响应格式无效");
  }
  return { path: value.path, name: value.name, title: value.title };
}

// A missing or unrecognized kind falls back to directory so the WebUI keeps
// the richer navigation UI when the server contract is uncertain.
function parsePreviewKind(value: unknown): PreviewKind {
  if (value === "single" || value === "directory" || value === "workspace") {
    return value;
  }
  return "directory";
}

function parseDocumentRef(payload: unknown): DocumentRef | null {
  if (payload === undefined || payload === null) {
    return null;
  }
  if (
    !isRecord(payload) ||
    typeof payload.root !== "string" ||
    typeof payload.path !== "string"
  ) {
    throw new Error("文件列表响应格式无效");
  }
  return { root: payload.root, path: payload.path };
}

function parseFrontMatter(payload: unknown): FrontMatter | null {
  if (payload === undefined || payload === null) {
    return null;
  }
  if (!isRecord(payload) || !Array.isArray(payload.entries)) {
    throw new Error("文档响应格式无效");
  }
  const entries = payload.entries.map((entry) => {
    if (
      !isRecord(entry) ||
      typeof entry.key !== "string" ||
      typeof entry.value !== "string"
    ) {
      throw new Error("文档响应格式无效");
    }
    return { key: entry.key, value: entry.value };
  });
  const result: FrontMatter = { entries };
  if (payload.date !== undefined && payload.date !== null) {
    if (typeof payload.date !== "string") {
      throw new Error("文档响应格式无效");
    }
    result.date = payload.date;
  }
  if (payload.tags !== undefined && payload.tags !== null) {
    if (
      !Array.isArray(payload.tags) ||
      !payload.tags.every((tag) => typeof tag === "string")
    ) {
      throw new Error("文档响应格式无效");
    }
    result.tags = payload.tags;
  }
  return result;
}

function parseTOC(payload: unknown): TocItem[] {
  // The server always sends toc as an array; accept a missing field gracefully
  // (treat as empty) but still validate the shape of every entry so a malformed
  // response can never reach the UI as an untrusted TocItem[].
  if (payload === undefined || payload === null) {
    return [];
  }
  if (!Array.isArray(payload)) {
    throw new Error("文档响应格式无效");
  }
  return payload.map((value) => {
    if (
      !isRecord(value) ||
      typeof value.level !== "number" ||
      typeof value.id !== "string" ||
      typeof value.text !== "string"
    ) {
      throw new Error("文档响应格式无效");
    }
    return { level: value.level, id: value.id, text: value.text };
  });
}

function parseDocument(payload: unknown): DocumentResponse {
  if (
    !isRecord(payload) ||
    typeof payload.path !== "string" ||
    typeof payload.title !== "string" ||
    typeof payload.html !== "string"
  ) {
    throw new Error("文档响应格式无效");
  }
  return {
    path: payload.path,
    title: payload.title,
    html: payload.html,
    frontmatter: parseFrontMatter(payload.frontmatter),
    toc: parseTOC(payload.toc),
  };
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

// The root kind decides how the client joins a local absolute path, so a
// missing or unrecognized value is a broken contract rather than a default.
function isRootKind(value: unknown): value is RootKind {
  return value === "file" || value === "directory";
}

export const browserAPI: PreviewAPI = {
  async listFiles(signal) {
    return parseFileList(await requestJSON("/api/files", signal));
  },
  async getDocument(path, signal) {
    const query = new URLSearchParams({ path });
    return parseDocument(
      await requestJSON(`/api/document?${query.toString()}`, signal),
    );
  },
  async getMarkdown(path, signal) {
    return requestText(markdownURL(path), signal);
  },
};
