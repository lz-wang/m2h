import { appPath } from "./lib/base-path";
// model.ts imports only types from this module, so pulling its URL builder in
// here creates no runtime cycle — and keeps every /raw/ address shaped by one
// encoder shared with the sidebar and toolbar links.
import { markdownURL } from "./model";

// FileSummary is the listing's topology-only entry: an addressable path and
// its display name. The listing never reads Markdown content, so title and
// description arrive separately — see FileMetadata and getFileMetadata.
export interface FileSummary {
  path: string;
  name: string;
}

// FileMetadata is the on-demand display metadata of one document, fetched
// per file when the sidebar actually shows it — the per-file complement to
// the (topology-only) file listing.
export interface FileMetadata {
  title: string;
  // The frontmatter description, when the document declares one. Absent —
  // not empty — is the "no description" state, so the tooltip can treat it
  // uniformly.
  description?: string;
}

// RootSummary groups one preview root's documents. Files carry root-relative
// paths; in a multi-root workspace the root id prefixes the addressable
// (virtual) document path, so identity stays unique across roots. The summary
// carries only logical document information: the server's filesystem layout
// never crosses the API, so the browser can never learn it.
export interface RootSummary {
  id: string;
  name: string;
  files: FileSummary[];
}

// PreviewKind reports what the server is previewing, so the WebUI can hide
// file navigation when there is nothing to navigate: one file, one directory,
// or a workspace of several roots.
export type PreviewKind = "single" | "directory" | "workspace";

export interface FileListResponse {
  kind: PreviewKind;
  roots: RootSummary[];
  version: string;
}

export interface RecentDocument {
  path: string;
  title: string;
  date: string;
  dateSource: "frontmatter" | "mtime";
}

export interface RecentResponse {
  created: RecentDocument[];
  updated: RecentDocument[];
}

export interface FrontMatterEntry {
  key: string;
  value: string;
}

// The normalized metadata derivations the server computed from the raw
// frontmatter: entries stay the verbatim table, while created/updated/date/tags
// are the only fields the WebUI is allowed to interpret (never raw keys).
export interface FrontMatter {
  entries: FrontMatterEntry[];
  created?: string;
  updated?: string;
  date?: string;
  tags?: string[];
}

export interface TocItem {
  level: number;
  id: string;
  text: string;
}

// DocumentRef is the persistent document identity: the root's stored UUID
// plus the root-relative path. Authenticated deployments attach it (and the
// document hash) to every document response; the default mode omits both.
export interface DocumentRef {
  rootId: string;
  path: string;
}

export interface DocumentResponse {
  path: string;
  title: string;
  html: string;
  frontmatter: FrontMatter | null;
  toc: TocItem[];
  documentRef?: DocumentRef;
  // "sha256:…" of the raw Markdown source bytes — the version annotations
  // are pinned against.
  documentHash?: string;
}

// SearchHeading locates the section a full-text match belongs to. The id is
// the same anchor id the rendered document uses, so a hit can deep-link to
// the exact heading.
export interface SearchHeading {
  id: string;
  text: string;
}

// SearchResult is the single best match of one document. Path carries the
// addressable (virtual) document path — the same identity /doc/ and the
// sidebar use — so the WebUI can select it without composing root ids.
// Snippet is a plain-text excerpt (empty for metadata-only matches) and
// heading is omitted when the match has no section.
export interface SearchResult {
  path: string;
  title: string;
  snippet?: string;
  heading?: SearchHeading;
  tabId?: string;
}

export interface SearchResponse {
  query: string;
  results: SearchResult[];
}

export interface PreviewAPI {
  listFiles(signal?: AbortSignal): Promise<FileListResponse>;
  getRecent(signal?: AbortSignal): Promise<RecentResponse>;
  getDocument(path: string, signal?: AbortSignal): Promise<DocumentResponse>;
  // Fetches one document's sidebar display metadata (title, description)
  // from /api/file-metadata on demand. Callers dedupe; the endpoint reads
  // exactly the one named Markdown file.
  getFileMetadata(path: string, signal?: AbortSignal): Promise<FileMetadata>;
  // Fetches the document's original Markdown source (frontmatter included)
  // from /raw/<virtual-path> on demand — sharing keeps it out of every
  // /api/document response until the reader actually asks for the full text.
  getMarkdown(path: string, signal?: AbortSignal): Promise<string>;
  // Runs a workspace-wide full-text query. The server scans the publishable
  // files at request time; the caller aborts superseded queries via signal.
  search(query: string, signal?: AbortSignal): Promise<SearchResponse>;
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
    reportUnauthorized(url, response.status);
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
    reportUnauthorized(url, response.status);
    throw new APIError(
      response.status,
      response.statusText || `HTTP ${response.status}`,
    );
  }
  return response.text();
}

// unauthorizedEventName announces that a protected request came back 401 —
// a session that expired mid-reading. The auth gate listens and swaps the
// reader for the login page. The /api/auth/* endpoints are exempt: a 401
// from "who am I" is the expected anonymous answer, not a session loss.
export const unauthorizedEventName = "m2h:unauthorized";

function reportUnauthorized(url: string, status: number): void {
  if (status !== 401 || url.includes("/api/auth/")) {
    return;
  }
  window.dispatchEvent(new CustomEvent(unauthorizedEventName));
}

// SiteConfig is the deployment's capability description — public in every
// mode because the WebUI needs it before it can even show a login page. It
// deliberately carries no user or deployment details.
export interface SiteConfig {
  authMode: AuthMode;
  features: { annotations: boolean; tokens: boolean };
}

export type AuthMode = "none" | "local" | "ldap";

export interface AuthIdentity {
  username: string;
  displayName: string;
  provider: string;
}

function parseSiteConfig(payload: unknown): SiteConfig {
  if (!isRecord(payload) || typeof payload.authMode !== "string") {
    throw new Error("站点配置响应格式无效");
  }
  const authMode = parseAuthMode(payload.authMode);
  const features = isRecord(payload.features) ? payload.features : {};
  return {
    authMode,
    features: {
      annotations: features.annotations === true,
      tokens: features.tokens === true,
    },
  };
}

function parseAuthMode(value: string): AuthMode {
  if (value === "local" || value === "ldap" || value === "none") {
    return value;
  }
  return "none";
}

function parseIdentity(payload: unknown): AuthIdentity {
  if (
    !isRecord(payload) ||
    typeof payload.username !== "string" ||
    typeof payload.displayName !== "string" ||
    typeof payload.provider !== "string"
  ) {
    throw new Error("会话响应格式无效");
  }
  return {
    username: payload.username,
    displayName: payload.displayName,
    provider: payload.provider,
  };
}

// fetchSiteConfig loads the capability description. Public in every mode.
export async function fetchSiteConfig(
  signal?: AbortSignal,
): Promise<SiteConfig> {
  return parseSiteConfig(await requestJSON(appPath("/api/config"), signal));
}

// fetchCurrentIdentity answers the session check: the identity when the
// session is valid, null when it is absent or expired.
export async function fetchCurrentIdentity(
  signal?: AbortSignal,
): Promise<AuthIdentity | null> {
  try {
    return parseIdentity(await requestJSON(appPath("/api/auth/me"), signal));
  } catch (error) {
    if (error instanceof APIError && error.status === 401) {
      return null;
    }
    throw error;
  }
}

// loginRequest submits credentials. Same-origin fetch sends the Origin
// header the server's CSRF check expects; the response carries the session
// cookie. Throws APIError 401 (bad credentials) or 429 (rate limited).
export async function loginRequest(
  username: string,
  password: string,
  signal?: AbortSignal,
): Promise<AuthIdentity> {
  const response = await fetch(appPath("/api/auth/login"), {
    method: "POST",
    headers: { "Content-Type": "application/json", Accept: "application/json" },
    body: JSON.stringify({ username, password }),
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
  return parseIdentity(payload);
}

// logoutRequest destroys the current session.
export async function logoutRequest(signal?: AbortSignal): Promise<void> {
  const response = await fetch(appPath("/api/auth/logout"), {
    method: "POST",
    headers: { Accept: "application/json" },
    signal,
  });
  if (!response.ok) {
    throw new APIError(
      response.status,
      response.statusText || `HTTP ${response.status}`,
    );
  }
}

// Annotation wire types. The selector mirrors the server's model: quote
// and position over the browser-normalized reading text, in Unicode code
// points.
export interface AnnotationSelectorData {
  quote: { exact: string; prefix: string; suffix: string };
  position: { start: number; end: number };
  headingId?: string;
  tabId?: string;
  blockHint?: string;
  version: 1;
}

export const ANNOTATION_COLORS = ["blue", "green", "amber"] as const;
export type AnnotationColor = (typeof ANNOTATION_COLORS)[number];
export const DEFAULT_ANNOTATION_COLOR: AnnotationColor = "blue";

export interface AnnotationItem {
  id: string;
  authorId: string;
  authorName: string;
  comment: string;
  color: AnnotationColor;
  quote: string;
  selector: AnnotationSelectorData;
  documentHash: string;
  mine: boolean;
  createdAt: number;
  updatedAt: number;
}

function parseAnnotationList(payload: unknown): AnnotationItem[] {
  if (!isRecord(payload) || !Array.isArray(payload.annotations)) {
    throw new Error("标注列表响应格式无效");
  }
  return payload.annotations.map(parseAnnotationItem);
}

function parseAnnotationItem(value: unknown): AnnotationItem {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    typeof value.authorId !== "string" ||
    typeof value.authorName !== "string" ||
    typeof value.comment !== "string" ||
    typeof value.quote !== "string" ||
    typeof value.documentHash !== "string" ||
    typeof value.mine !== "boolean" ||
    typeof value.createdAt !== "number" ||
    typeof value.updatedAt !== "number" ||
    !isRecord(value.selector)
  ) {
    throw new Error("标注响应格式无效");
  }
  const selector = value.selector;
  const color = value.color ?? DEFAULT_ANNOTATION_COLOR;
  if (
    (color !== "blue" && color !== "green" && color !== "amber") ||
    !isRecord(selector.quote) ||
    typeof selector.quote.exact !== "string" ||
    typeof selector.quote.prefix !== "string" ||
    typeof selector.quote.suffix !== "string" ||
    !isRecord(selector.position) ||
    typeof selector.position.start !== "number" ||
    typeof selector.position.end !== "number" ||
    selector.version !== 1
  ) {
    throw new Error("标注响应格式无效");
  }
  const parsed: AnnotationItem = {
    id: value.id,
    authorId: value.authorId,
    authorName: value.authorName,
    comment: value.comment,
    color,
    quote: value.quote,
    documentHash: value.documentHash,
    mine: value.mine,
    createdAt: value.createdAt,
    updatedAt: value.updatedAt,
    selector: {
      quote: {
        exact: selector.quote.exact,
        prefix: selector.quote.prefix,
        suffix: selector.quote.suffix,
      },
      position: {
        start: selector.position.start,
        end: selector.position.end,
      },
      version: selector.version,
    },
  };
  if (typeof selector.headingId === "string") {
    parsed.selector.headingId = selector.headingId;
  }
  if (typeof selector.tabId === "string") {
    parsed.selector.tabId = selector.tabId;
  }
  if (typeof selector.blockHint === "string") {
    parsed.selector.blockHint = selector.blockHint;
  }
  return parsed;
}

async function requestMutation(
  url: string,
  method: "POST" | "DELETE" | "PATCH",
  body?: unknown,
): Promise<unknown> {
  const response = await fetch(url, {
    method,
    headers: {
      Accept: "application/json",
      ...(body === undefined ? {} : { "Content-Type": "application/json" }),
    },
    ...(body === undefined ? {} : { body: JSON.stringify(body) }),
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

// listAnnotations fetches one document's annotations (oldest first).
export async function listAnnotations(
  path: string,
  signal?: AbortSignal,
): Promise<AnnotationItem[]> {
  const query = new URLSearchParams({ path });
  return parseAnnotationList(
    await requestJSON(appPath(`/api/annotations?${query.toString()}`), signal),
  );
}

// createAnnotation posts a new annotation pinned to the given document
// identity and content version. A 409 means the document changed since the
// caller loaded it.
export async function createAnnotation(
  documentRef: DocumentRef,
  documentHash: string,
  selector: AnnotationSelectorData,
  comment: string,
  color: AnnotationColor = DEFAULT_ANNOTATION_COLOR,
): Promise<AnnotationItem> {
  return parseAnnotationItem(
    await requestMutation(appPath("/api/annotations"), "POST", {
      documentRef,
      documentHash,
      selector,
      comment,
      color,
    }),
  );
}

// deleteAnnotation removes one of the caller's own annotations.
export async function deleteAnnotation(id: string): Promise<void> {
  await requestMutation(appPath(`/api/annotations/${id}`), "DELETE");
}

export async function updateAnnotationComment(
  id: string,
  comment: string,
): Promise<AnnotationItem> {
  return parseAnnotationItem(
    await requestMutation(appPath(`/api/annotations/${id}/comment`), "PATCH", {
      comment,
    }),
  );
}

// reanchorAnnotation confirms a new selector for an annotation the caller
// owns, after the document moved under it. The document hash states the
// version the new selection was made on; a 409 means the document moved
// again in between.
export async function reanchorAnnotation(
  id: string,
  documentHash: string,
  selector: AnnotationSelectorData,
): Promise<AnnotationItem> {
  return parseAnnotationItem(
    await requestMutation(appPath(`/api/annotations/${id}/anchor`), "PATCH", {
      documentHash,
      selector,
    }),
  );
}

// API token wire types.
export interface TokenScopeData {
  permissions: string[];
  resource: {
    type: "all_published" | "root" | "document";
    rootId?: string;
    path?: string;
  };
}

export interface TokenItem {
  id: string;
  name: string;
  kind: "temporary" | "persistent";
  scope: TokenScopeData;
  createdAt: number;
  expiresAt: number;
  lastUsedAt: number;
  documentHash?: string;
  // The plaintext secret appears exactly once, in the creation response.
  secret?: string;
}

function parseTokenItem(value: unknown): TokenItem {
  if (
    !isRecord(value) ||
    typeof value.id !== "string" ||
    typeof value.name !== "string" ||
    typeof value.kind !== "string" ||
    typeof value.createdAt !== "number" ||
    typeof value.expiresAt !== "number" ||
    typeof value.lastUsedAt !== "number" ||
    !isRecord(value.scope) ||
    !Array.isArray(value.scope.permissions) ||
    !isRecord(value.scope.resource)
  ) {
    throw new Error("令牌响应格式无效");
  }
  const resource = value.scope.resource;
  if (typeof resource.type !== "string") {
    throw new Error("令牌响应格式无效");
  }
  const token: TokenItem = {
    id: value.id,
    name: value.name,
    kind: value.kind === "temporary" ? "temporary" : "persistent",
    scope: {
      permissions: value.scope.permissions.filter(
        (item): item is string => typeof item === "string",
      ),
      resource: {
        type: resource.type as TokenScopeData["resource"]["type"],
      },
    },
    createdAt: value.createdAt,
    expiresAt: value.expiresAt,
    lastUsedAt: value.lastUsedAt,
  };
  if (typeof resource.rootId === "string") {
    token.scope.resource.rootId = resource.rootId;
  }
  if (typeof resource.path === "string") {
    token.scope.resource.path = resource.path;
  }
  if (typeof value.documentHash === "string") {
    token.documentHash = value.documentHash;
  }
  if (typeof value.secret === "string") {
    token.secret = value.secret;
  }
  return token;
}

// listTokens answers the caller's own tokens (metadata only).
export async function listTokens(signal?: AbortSignal): Promise<TokenItem[]> {
  const payload = await requestJSON(appPath("/api/tokens"), signal);
  if (!isRecord(payload) || !Array.isArray(payload.tokens)) {
    throw new Error("令牌列表响应格式无效");
  }
  return payload.tokens.map(parseTokenItem);
}

// createTokenInput is the creation request's shape.
export interface CreateTokenInput {
  kind: "temporary" | "persistent";
  name: string;
  expiresInDays?: number;
  permanent?: boolean;
  scope: TokenScopeData;
  documentHash?: string;
}

// createToken mints a token; the response carries the secret exactly once.
export async function createToken(input: CreateTokenInput): Promise<TokenItem> {
  return parseTokenItem(
    await requestMutation(appPath("/api/tokens"), "POST", input),
  );
}

// revokeToken tombstones one of the caller's own tokens.
export async function revokeToken(id: string): Promise<void> {
  await requestMutation(appPath(`/api/tokens/${id}`), "DELETE");
}

function parseRecentDocument(value: unknown): RecentDocument {
  if (
    !isRecord(value) ||
    typeof value.path !== "string" ||
    value.path === "" ||
    typeof value.title !== "string" ||
    typeof value.date !== "string" ||
    !/^\d{4}-\d{2}-\d{2}$/.test(value.date) ||
    (value.dateSource !== "frontmatter" && value.dateSource !== "mtime")
  ) {
    throw new Error("最近文档响应格式无效");
  }
  return {
    path: value.path,
    title: value.title,
    date: value.date,
    dateSource: value.dateSource,
  };
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
      !Array.isArray(value.files)
    ) {
      throw new Error("文件列表响应格式无效");
    }
    const root: RootSummary = {
      id: value.id,
      name: value.name,
      files: value.files.map(parseFileSummary),
    };
    return root;
  });
  return {
    kind: parsePreviewKind(payload.kind),
    roots,
    version: payload.version,
  };
}

function parseFileSummary(value: unknown): FileSummary {
  if (
    !isRecord(value) ||
    typeof value.path !== "string" ||
    typeof value.name !== "string"
  ) {
    throw new Error("文件条目响应格式无效");
  }
  // Topology only: anything else the payload carries is ignored, matching
  // the listing's metadata-free contract.
  return { path: value.path, name: value.name };
}

// A missing or unrecognized kind falls back to directory so the WebUI keeps
// the richer navigation UI when the server contract is uncertain.
function parsePreviewKind(value: unknown): PreviewKind {
  if (value === "single" || value === "directory" || value === "workspace") {
    return value;
  }
  return "directory";
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
  const created = optionalString(payload, "created");
  if (created !== undefined) {
    result.created = created;
  }
  const updated = optionalString(payload, "updated");
  if (updated !== undefined) {
    result.updated = updated;
  }
  const date = optionalString(payload, "date");
  if (date !== undefined) {
    result.date = date;
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

// A missing or null optional string field is accepted as absent; anything but
// a string is a malformed response.
function optionalString(
  payload: Record<string, unknown>,
  key: string,
): string | undefined {
  const value = payload[key];
  if (value === undefined || value === null) {
    return undefined;
  }
  if (typeof value !== "string") {
    throw new Error("文档响应格式无效");
  }
  return value;
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
  const document: DocumentResponse = {
    path: payload.path,
    title: payload.title,
    html: payload.html,
    frontmatter: parseFrontMatter(payload.frontmatter),
    toc: parseTOC(payload.toc),
  };
  if (payload.documentRef !== undefined && payload.documentRef !== null) {
    const ref = payload.documentRef;
    if (
      !isRecord(ref) ||
      typeof ref.rootId !== "string" ||
      typeof ref.path !== "string"
    ) {
      throw new Error("文档响应格式无效");
    }
    document.documentRef = { rootId: ref.rootId, path: ref.path };
  }
  if (payload.documentHash !== undefined && payload.documentHash !== null) {
    if (typeof payload.documentHash !== "string") {
      throw new Error("文档响应格式无效");
    }
    document.documentHash = payload.documentHash;
  }
  return document;
}

function parseFileMetadata(payload: unknown): FileMetadata {
  if (!isRecord(payload) || typeof payload.title !== "string") {
    throw new Error("文件元数据响应格式无效");
  }
  const metadata: FileMetadata = { title: payload.title };
  if (payload.description !== undefined) {
    if (typeof payload.description !== "string") {
      throw new Error("文件元数据响应格式无效");
    }
    metadata.description = payload.description;
  }
  return metadata;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function parseSearchResponse(payload: unknown): SearchResponse {
  if (
    !isRecord(payload) ||
    typeof payload.query !== "string" ||
    !Array.isArray(payload.results)
  ) {
    throw new Error("搜索响应格式无效");
  }
  const results = payload.results.map((value) => {
    if (
      !isRecord(value) ||
      typeof value.path !== "string" ||
      typeof value.title !== "string"
    ) {
      throw new Error("搜索响应格式无效");
    }
    const result: SearchResult = { path: value.path, title: value.title };
    if (value.tabId !== undefined && value.tabId !== null) {
      if (typeof value.tabId !== "string") throw new Error("搜索响应格式无效");
      result.tabId = value.tabId;
    }
    if (value.snippet !== undefined && value.snippet !== null) {
      if (typeof value.snippet !== "string") {
        throw new Error("搜索响应格式无效");
      }
      result.snippet = value.snippet;
    }
    if (value.heading !== undefined && value.heading !== null) {
      if (
        !isRecord(value.heading) ||
        typeof value.heading.id !== "string" ||
        typeof value.heading.text !== "string"
      ) {
        throw new Error("搜索响应格式无效");
      }
      result.heading = { id: value.heading.id, text: value.heading.text };
    }
    return result;
  });
  return { query: payload.query, results };
}

export const browserAPI: PreviewAPI = {
  async listFiles(signal) {
    return parseFileList(await requestJSON(appPath("/api/files"), signal));
  },
  async getRecent(signal) {
    const payload = await requestJSON(appPath("/api/recent"), signal);
    if (
      !isRecord(payload) ||
      !Array.isArray(payload.created) ||
      !Array.isArray(payload.updated)
    ) {
      throw new Error("最近文档响应格式无效");
    }
    return {
      created: payload.created.map(parseRecentDocument),
      updated: payload.updated.map(parseRecentDocument),
    };
  },
  async getDocument(path, signal) {
    const query = new URLSearchParams({ path });
    return parseDocument(
      await requestJSON(appPath(`/api/document?${query.toString()}`), signal),
    );
  },
  async getFileMetadata(path, signal) {
    const query = new URLSearchParams({ path });
    return parseFileMetadata(
      await requestJSON(
        appPath(`/api/file-metadata?${query.toString()}`),
        signal,
      ),
    );
  },
  async getMarkdown(path, signal) {
    return requestText(markdownURL(path), signal);
  },
  async search(query, signal) {
    const params = new URLSearchParams({ q: query });
    return parseSearchResponse(
      await requestJSON(appPath(`/api/search?${params.toString()}`), signal),
    );
  },
};
