import type { DocumentRef, FileSummary, RootSummary } from "./api";

export type Mode = "light" | "dark" | "auto";
// ResolvedMode is the concrete light/dark the UI settles on after resolving
// "auto" against prefers-color-scheme. Markdown rendering surfaces that bake
// colors in at render time (Mermaid) key off this rather than Mode so an OS
// theme switch in "auto" still propagates without an explicit user action.
export type ResolvedMode = "light" | "dark";
export type DocumentWidth = "standard" | "wide" | "full";

export interface RouteState {
  path: string | null;
  mode: Mode;
  width: DocumentWidth;
  toc: boolean;
  hash: string;
}

// ViewOptions bundles the URL-backed view preferences so routeURL stays stable
// as new preferences are added instead of growing a positional argument list.
export interface ViewOptions {
  mode: Mode;
  width: DocumentWidth;
  toc: boolean;
}

export interface DirectoryNode {
  type: "directory";
  name: string;
  path: string;
  children: TreeNode[];
}

export interface FileNode {
  type: "file";
  name: string;
  path: string;
  file: FileSummary;
}

export type TreeNode = DirectoryNode | FileNode;

export function readRoute(
  pathname: string,
  search: string,
  hash = "",
): RouteState {
  const parameters = new URLSearchParams(search);
  const requestedMode = parameters.get("mode");
  const mode: Mode = isMode(requestedMode) ? requestedMode : "auto";
  const requestedWidth = parameters.get("width");
  const width: DocumentWidth = isDocumentWidth(requestedWidth)
    ? requestedWidth
    : "standard";
  // TOC defaults to true; only the explicit "false" value disables it. Any
  // other value (including a missing parameter) keeps it on.
  const toc = parameters.get("toc") !== "false";
  if (!pathname.startsWith("/doc/")) {
    return { path: null, mode, width, toc, hash: normalizeHash(hash) };
  }
  const encoded = pathname.slice("/doc/".length);
  if (encoded === "") {
    return { path: null, mode, width, toc, hash: normalizeHash(hash) };
  }
  try {
    return {
      path: decodeURIComponent(encoded),
      mode,
      width,
      toc,
      hash: normalizeHash(hash),
    };
  } catch {
    return { path: null, mode, width, toc, hash: normalizeHash(hash) };
  }
}

export function routeURL(
  path: string | null,
  options: ViewOptions,
  hash = "",
): string {
  const parameters = new URLSearchParams();
  if (options.mode !== "auto") {
    parameters.set("mode", options.mode);
  }
  if (options.width !== "standard") {
    parameters.set("width", options.width);
  }
  if (!options.toc) {
    parameters.set("toc", "false");
  }
  const query = parameters.toString();
  const search = query === "" ? "" : `?${query}`;
  const suffix = normalizeHash(hash);
  if (path === null) {
    return `/${search}${suffix}`;
  }
  return `/doc/${encodeVirtualPath(path)}${search}${suffix}`;
}

// encodeVirtualPath percent-encodes each slash-separated segment of a virtual
// document key separately, so a file named "a b.md", "#tag.md" or "50%.md"
// survives the address bar while the structural slashes stay literal. Every
// /doc/ and /raw/ URL in the app is built through this one encoder.
export function encodeVirtualPath(path: string): string {
  return path
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");
}

// The shareable addresses of one document. Unlike routeURL these carry no
// mode/width/toc parameters — those are the sender's personal UI preferences —
// but they do keep the heading hash so a shared link can land on the exact
// section the sender was reading.
export function documentURL(path: string, hash = ""): string {
  return `/doc/${encodeVirtualPath(path)}${normalizeHash(hash)}`;
}

// The browser-addressable source URL of one document's original Markdown.
export function markdownURL(path: string): string {
  return `/raw/${encodeVirtualPath(path)}`;
}

// Resolves a share URL against the current origin. Built from the URL API so
// Unicode, spaces, "#" and "%" in paths are re-encoded correctly — never hand-
// concatenated with location.origin.
export function absoluteURL(url: string, origin: string): string {
  return new URL(url, origin).href;
}

// Virtual document keys: with several roots every document is addressed as
// "<rootId>/<path>" — URL, document API, scroll storage and sidebar selection
// all share the key — while a single root keeps bare relative paths so
// existing /doc/foo.md URLs stay valid.
export function rootFiles(roots: RootSummary[]): FileSummary[] {
  if (roots.length <= 1) {
    return roots[0]?.files ?? [];
  }
  return roots.flatMap((root) =>
    root.files.map((file) => ({ ...file, path: `${root.id}/${file.path}` })),
  );
}

// Compose the virtual key of the server's default document; "" when the
// workspace has no documents at all.
export function rootDocumentKey(
  ref: DocumentRef | null,
  multiRoot: boolean,
): string {
  if (ref === null) {
    return "";
  }
  return multiRoot ? `${ref.root}/${ref.path}` : ref.path;
}

// One document's place in the workspace: which root serves it, its summary
// entry, and its root-relative path. Callers never re-parse the multi-root
// "<rootId>/<path>" convention themselves.
export interface ResolvedDocumentLocation {
  root: RootSummary;
  file: FileSummary;
  relativePath: string;
}

// Maps a virtual document key back onto its root and root-relative path:
// with one root the whole key is relative; with several, the first segment
// must name a root. Returns null when the key addresses nothing known —
// the caller decides whether that means "no selection" or "gone".
export function resolveDocumentLocation(
  roots: RootSummary[],
  virtualPath: string | null,
): ResolvedDocumentLocation | null {
  if (virtualPath === null) {
    return null;
  }
  if (roots.length <= 1) {
    const root = roots[0];
    if (root === undefined) {
      return null;
    }
    const file = root.files.find((entry) => entry.path === virtualPath);
    if (file === undefined) {
      return null;
    }
    return { root, file, relativePath: virtualPath };
  }
  const separator = virtualPath.indexOf("/");
  if (separator <= 0) {
    return null;
  }
  const root = roots.find(
    (entry) => entry.id === virtualPath.slice(0, separator),
  );
  if (root === undefined) {
    return null;
  }
  const relativePath = virtualPath.slice(separator + 1);
  const file = root.files.find((entry) => entry.path === relativePath);
  if (file === undefined) {
    return null;
  }
  return { root, file, relativePath };
}

// Join a server-reported absolute root path with a root-relative slash path
// using the SERVER machine's separator: the browser may run on a different
// machine, so the platform's native format must come from the API, never from
// the client's own OS sniffing.
export function localPath(
  root: string,
  relative: string,
  separator: string,
): string {
  if (relative === "") {
    return root;
  }
  const suffix = relative.split("/").join(separator);
  if (root.endsWith(separator)) {
    return `${root}${suffix}`;
  }
  return `${root}${separator}${suffix}`;
}

// The document's absolute path on the server machine. A file root's
// absolutePath already names the file itself, so the relative path is not
// appended again — the root kind keeps single-file and mixed workspaces both
// correct.
export function localDocumentPath(
  root: RootSummary,
  relativePath: string,
): string {
  if (root.kind === "file") {
    return root.absolutePath;
  }
  return localPath(root.absolutePath, relativePath, root.pathSeparator);
}

export function chooseDocument(
  files: FileSummary[],
  requested: string | null,
  defaultPath: string,
): string | null {
  if (requested !== null && files.some((file) => file.path === requested)) {
    return requested;
  }
  if (defaultPath !== "" && files.some((file) => file.path === defaultPath)) {
    return defaultPath;
  }
  return files[0]?.path ?? null;
}

export function buildTree(files: FileSummary[]): TreeNode[] {
  const root: DirectoryNode = {
    type: "directory",
    name: "",
    path: "",
    children: [],
  };
  for (const file of files) {
    const segments = file.path.split("/");
    let parent = root;
    for (let index = 0; index < segments.length - 1; index += 1) {
      const name = segments[index];
      if (name === undefined) {
        continue;
      }
      const directoryPath = segments.slice(0, index + 1).join("/");
      const existing = parent.children.find(
        (node): node is DirectoryNode =>
          node.type === "directory" && node.name === name,
      );
      if (existing !== undefined) {
        parent = existing;
        continue;
      }
      const directory: DirectoryNode = {
        type: "directory",
        name,
        path: directoryPath,
        children: [],
      };
      parent.children.push(directory);
      parent = directory;
    }
    parent.children.push({
      type: "file",
      name: file.name,
      path: file.path,
      file,
    });
  }
  sortTree(root.children);
  return root.children;
}

export function ancestorDirectories(path: string | null): string[] {
  if (path === null) {
    return [];
  }
  const segments = path.split("/");
  return segments
    .slice(0, -1)
    .map((_, index) => segments.slice(0, index + 1).join("/"));
}

function sortTree(nodes: TreeNode[]): void {
  nodes.sort((left, right) => {
    if (left.type !== right.type) {
      return left.type === "directory" ? -1 : 1;
    }
    return left.name.localeCompare(right.name, undefined, { numeric: true });
  });
  for (const node of nodes) {
    if (node.type === "directory") {
      sortTree(node.children);
    }
  }
}

function isMode(value: string | null): value is Mode {
  return value === "light" || value === "dark" || value === "auto";
}

function isDocumentWidth(value: string | null): value is DocumentWidth {
  return value === "standard" || value === "wide" || value === "full";
}

function normalizeHash(hash: string): string {
  if (hash === "" || hash === "#") {
    return "";
  }
  return hash.startsWith("#") ? hash : `#${hash}`;
}

// encodeHeadingHash wraps a heading id into the URL fragment form the reader
// writes and restores: a leading "#" plus percent-encoding so Unicode and
// punctuation survive a round-trip through the address bar intact. The empty
// string means "no fragment"; callers clear the hash by passing "" rather than
// building it here.
export function encodeHeadingHash(id: string): string {
  return `#${encodeURIComponent(id)}`;
}

// decodeHeadingHash is the inverse: strip the leading "#" and percent-decode,
// keeping the literal fragment when it is not valid percent encoding. Returns
// "" for an empty/absent fragment.
export function decodeHeadingHash(hash: string): string {
  const encoded = hash.startsWith("#") ? hash.slice(1) : hash;
  if (encoded === "") {
    return "";
  }
  try {
    return decodeURIComponent(encoded);
  } catch {
    return encoded;
  }
}
