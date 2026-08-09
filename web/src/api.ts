export interface FileSummary {
  path: string;
  name: string;
  title: string;
}

export interface FileListResponse {
  files: FileSummary[];
  defaultPath: string;
}

export interface DocumentResponse {
  path: string;
  title: string;
  html: string;
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

function parseDocument(payload: unknown): DocumentResponse {
  if (
    !isRecord(payload) ||
    typeof payload.path !== "string" ||
    typeof payload.title !== "string" ||
    typeof payload.html !== "string"
  ) {
    throw new Error("文档响应格式无效");
  }
  return { path: payload.path, title: payload.title, html: payload.html };
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
