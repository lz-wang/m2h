import type { FileSummary } from "./api";

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
  const encoded = path
    .split("/")
    .map((segment) => encodeURIComponent(segment))
    .join("/");
  return `/doc/${encoded}${search}${suffix}`;
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
