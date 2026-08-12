import { useCallback, useEffect, useRef, useState } from "react";

import {
  APIError,
  browserAPI,
  type DocumentResponse,
  type FileSummary,
  type PreviewAPI,
  type PreviewKind,
} from "./api";
import {
  chooseDocument,
  type DocumentWidth,
  type Mode,
  readRoute,
  routeURL,
} from "./model";

export type PreviewPhase =
  | "loading-files"
  | "loading-document"
  | "ready"
  | "empty"
  | "error";

export interface DirectoryPreviewState {
  kind: PreviewKind;
  files: FileSummary[];
  selectedPath: string | null;
  document: DocumentResponse | null;
  mode: Mode;
  width: DocumentWidth;
  toc: boolean;
  phase: PreviewPhase;
  error: string | null;
  assetError: string | null;
  refresh(): Promise<void>;
  reloadCurrent(): Promise<void>;
  select(path: string, hash?: string): Promise<void>;
  setMode(mode: Mode): void;
  setWidth(width: DocumentWidth): void;
  setTOC(toc: boolean): void;
  reportAssetError(source: string): void;
  retry(): Promise<void>;
}

type HistoryAction = "push" | "replace" | "none";

export function useDirectoryPreview(
  api: PreviewAPI = browserAPI,
): DirectoryPreviewState {
  const initialRoute = useRef(
    readRoute(
      window.location.pathname,
      window.location.search,
      window.location.hash,
    ),
  );
  const [files, setFiles] = useState<FileSummary[]>([]);
  const [kind, setKind] = useState<PreviewKind>("directory");
  const [selectedPath, setSelectedPath] = useState<string | null>(null);
  const [documentResponse, setDocumentResponse] =
    useState<DocumentResponse | null>(null);
  const [mode, setModeState] = useState<Mode>(initialRoute.current.mode);
  const [width, setWidthState] = useState<DocumentWidth>(
    initialRoute.current.width,
  );
  const [toc, setTOCState] = useState<boolean>(initialRoute.current.toc);
  const [phase, setPhase] = useState<PreviewPhase>("loading-files");
  const [error, setError] = useState<string | null>(null);
  const [assetError, setAssetError] = useState<string | null>(null);
  const filesRef = useRef<FileSummary[]>([]);
  const defaultPathRef = useRef("");
  const selectedPathRef = useRef<string | null>(null);
  const modeRef = useRef<Mode>(initialRoute.current.mode);
  const widthRef = useRef<DocumentWidth>(initialRoute.current.width);
  const tocRef = useRef<boolean>(initialRoute.current.toc);
  const listController = useRef<AbortController | null>(null);
  const documentController = useRef<AbortController | null>(null);
  const documentRequest = useRef(0);

  const writeRoute = useCallback(
    (path: string | null, action: HistoryAction, hash = "") => {
      if (action === "none") {
        return;
      }
      const url = routeURL(
        path,
        {
          mode: modeRef.current,
          width: widthRef.current,
          toc: tocRef.current,
        },
        hash,
      );
      if (action === "push") {
        window.history.pushState(null, "", url);
      } else {
        window.history.replaceState(null, "", url);
      }
    },
    [],
  );

  const loadDocument = useCallback(
    async (path: string, historyAction: HistoryAction, hash = "") => {
      documentController.current?.abort();
      const controller = new AbortController();
      documentController.current = controller;
      documentRequest.current += 1;
      const request = documentRequest.current;
      selectedPathRef.current = path;
      setSelectedPath(path);
      setDocumentResponse(null);
      setAssetError(null);
      setError(null);
      setPhase("loading-document");
      writeRoute(path, historyAction, hash);
      try {
        const loaded = await api.getDocument(path, controller.signal);
        if (request !== documentRequest.current || controller.signal.aborted) {
          return;
        }
        if (loaded.path !== path) {
          throw new Error("文档响应路径与请求不一致");
        }
        setDocumentResponse(loaded);
        setPhase("ready");
      } catch (reason: unknown) {
        if (controller.signal.aborted || isAbortError(reason)) {
          return;
        }
        setDocumentResponse(null);
        setError(documentError(reason));
        setPhase("error");
      }
    },
    [api, writeRoute],
  );

  const loadFiles = useCallback(
    async (requested: string | null, requestedHash = "") => {
      listController.current?.abort();
      documentController.current?.abort();
      documentRequest.current += 1;
      const controller = new AbortController();
      listController.current = controller;
      setError(null);
      setAssetError(null);
      setPhase("loading-files");
      try {
        const loaded = await api.listFiles(controller.signal);
        if (controller.signal.aborted) {
          return;
        }
        filesRef.current = loaded.files;
        defaultPathRef.current = loaded.defaultPath;
        setFiles(loaded.files);
        setKind(loaded.kind);
        const target = chooseDocument(
          loaded.files,
          requested,
          loaded.defaultPath,
        );
        if (target === null) {
          selectedPathRef.current = null;
          setSelectedPath(null);
          setDocumentResponse(null);
          setPhase("empty");
          writeRoute(null, "replace");
          return;
        }
        const targetHash = target === requested ? requestedHash : "";
        await loadDocument(target, "replace", targetHash);
      } catch (reason: unknown) {
        if (controller.signal.aborted || isAbortError(reason)) {
          return;
        }
        setError("无法读取 Markdown 文件列表，请检查服务日志后重试。");
        setPhase("error");
      }
    },
    [api, loadDocument, writeRoute],
  );

  useEffect(() => {
    void loadFiles(initialRoute.current.path, initialRoute.current.hash);
    return () => {
      listController.current?.abort();
      documentController.current?.abort();
    };
  }, [loadFiles]);

  useEffect(() => {
    const handlePopState = () => {
      const route = readRoute(
        window.location.pathname,
        window.location.search,
        window.location.hash,
      );
      modeRef.current = route.mode;
      widthRef.current = route.width;
      tocRef.current = route.toc;
      setModeState(route.mode);
      setWidthState(route.width);
      setTOCState(route.toc);
      const target = chooseDocument(
        filesRef.current,
        route.path,
        defaultPathRef.current,
      );
      if (target === null) {
        selectedPathRef.current = null;
        setSelectedPath(null);
        setDocumentResponse(null);
        setPhase("empty");
        writeRoute(null, "replace");
        return;
      }
      const targetHash = target === route.path ? route.hash : "";
      const canonical = routeURL(
        target,
        {
          mode: route.mode,
          width: route.width,
          toc: route.toc,
        },
        targetHash,
      );
      const current = `${window.location.pathname}${window.location.search}${window.location.hash}`;
      if (canonical !== current) {
        window.history.replaceState(null, "", canonical);
      }
      void loadDocument(target, "none", targetHash);
    };
    window.addEventListener("popstate", handlePopState);
    return () => window.removeEventListener("popstate", handlePopState);
  }, [loadDocument, writeRoute]);

  useEffect(() => {
    applyTheme(mode);
    const media =
      mode === "auto" && typeof window.matchMedia === "function"
        ? window.matchMedia("(prefers-color-scheme: dark)")
        : null;
    const updateAutoTheme = () => applyTheme(mode, media?.matches ?? false);
    media?.addEventListener("change", updateAutoTheme);
    return () => media?.removeEventListener("change", updateAutoTheme);
  }, [mode]);

  useEffect(() => {
    document.title = documentResponse?.title ?? "m2h";
    if (documentResponse === null || window.location.hash === "") {
      return;
    }
    const frame = window.requestAnimationFrame(() => {
      const encodedID = window.location.hash.slice(1);
      let targetID = encodedID;
      try {
        targetID = decodeURIComponent(encodedID);
      } catch {
        // Keep the literal fragment when it is not valid percent encoding.
      }
      document.getElementById(targetID)?.scrollIntoView({ block: "start" });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [documentResponse]);

  const refresh = useCallback(async () => {
    await loadFiles(selectedPathRef.current, window.location.hash);
  }, [loadFiles]);

  // reloadCurrent re-fetches only the open document without touching history,
  // so a server-sent "document-changed" event can hot-swap the body while
  // preserving sidebar, theme, width, TOC and URL state.
  const reloadCurrent = useCallback(async () => {
    const path = selectedPathRef.current;
    if (path === null) {
      return;
    }
    await loadDocument(path, "none", window.location.hash);
  }, [loadDocument]);

  const select = useCallback(
    async (path: string, hash = "") => {
      if (!filesRef.current.some((file) => file.path === path)) {
        setError("所选文档已不在文件列表中，请重新加载后重试。");
        setPhase("error");
        return;
      }
      const action: HistoryAction =
        path === selectedPathRef.current ? "replace" : "push";
      await loadDocument(path, action, hash);
    },
    [loadDocument],
  );

  const setMode = useCallback(
    (nextMode: Mode) => {
      modeRef.current = nextMode;
      setModeState(nextMode);
      writeRoute(selectedPathRef.current, "replace", window.location.hash);
    },
    [writeRoute],
  );

  const setWidth = useCallback(
    (nextWidth: DocumentWidth) => {
      widthRef.current = nextWidth;
      setWidthState(nextWidth);
      writeRoute(selectedPathRef.current, "replace", window.location.hash);
    },
    [writeRoute],
  );

  const setTOC = useCallback(
    (nextTOC: boolean) => {
      tocRef.current = nextTOC;
      setTOCState(nextTOC);
      writeRoute(selectedPathRef.current, "replace", window.location.hash);
    },
    [writeRoute],
  );

  const reportAssetError = useCallback((source: string) => {
    setAssetError(
      source === "" ? "有附件加载失败。" : `附件加载失败：${source}`,
    );
  }, []);

  return {
    kind,
    files,
    selectedPath,
    document: documentResponse,
    mode,
    width,
    toc,
    phase,
    error,
    assetError,
    refresh,
    reloadCurrent,
    select,
    setMode,
    setWidth,
    setTOC,
    reportAssetError,
    retry: refresh,
  };
}

function applyTheme(mode: Mode, autoDark?: boolean): void {
  const root = document.documentElement;
  root.dataset.mode = mode;
  root.classList.remove("m2h-mode-light", "m2h-mode-dark", "m2h-mode-auto");
  root.classList.add(`m2h-mode-${mode}`);
  const systemDark =
    autoDark ??
    (typeof window.matchMedia === "function" &&
      window.matchMedia("(prefers-color-scheme: dark)").matches);
  const dark = mode === "dark" || (mode === "auto" && systemDark);
  root.classList.toggle("dark", dark);

  const existingStylesheet = document.getElementById("m2h-markdown-styles");
  const stylesheet =
    existingStylesheet instanceof HTMLLinkElement
      ? existingStylesheet
      : document.createElement("link");
  if (!(existingStylesheet instanceof HTMLLinkElement)) {
    stylesheet.id = "m2h-markdown-styles";
    stylesheet.rel = "stylesheet";
    document.head.append(stylesheet);
  }
  stylesheet.href = `/ui/markdown.css?mode=${mode}`;
}

function documentError(reason: unknown): string {
  if (reason instanceof APIError && reason.status === 404) {
    return "文档已被删除或不再符合当前筛选条件，请重新加载后重试。";
  }
  if (reason instanceof APIError && reason.status === 422) {
    return "Frontmatter 格式无效，请检查 YAML。";
  }
  return "无法读取该文档，请检查服务日志后重试。";
}

function isAbortError(reason: unknown): boolean {
  return reason instanceof DOMException && reason.name === "AbortError";
}
