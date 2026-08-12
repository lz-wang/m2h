export interface FileSummary {
  path: string;
  name: string;
  title: string;
}

export interface FileListResponse {
  files: FileSummary[];
  defaultPath: string;
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
    !Array.isArray(payload.files) ||
    typeof payload.defaultPath !== "string"
  ) {
    throw new Error("文件列表响应格式无效");
  }
  const files = payload.files.map((value) => {
    if (
      !isRecord(value) ||
      typeof value.path !== "string" ||
      typeof value.name !== "string" ||
      typeof value.title !== "string"
    ) {
      throw new Error("文件条目响应格式无效");
    }
    return { path: value.path, name: value.name, title: value.title };
  });
  return { files, defaultPath: payload.defaultPath };
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
