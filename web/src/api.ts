export interface FileSummary {
  path: string;
  name: string;
  title: string;
}

// RootSummary groups one preview root's documents. Files carry root-relative
// paths; in a multi-root workspace the root id prefixes the addressable
// (virtual) document path, so identity stays unique across roots.
// absolutePath is the server machine's canonical local path for the input and
// pathSeparator is that machine's separator — the browser may run elsewhere,
// so joining a native path must use the server-reported separator.
export interface RootSummary {
  id: string;
  name: string;
  absolutePath: string;
  pathSeparator: string;
  files: FileSummary[];
}

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
};
