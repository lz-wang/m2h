import { Menu } from "@base-ui/react/menu";
import {
  Check,
  ChevronRight,
  Columns3,
  Funnel,
  House,
  ImageOff,
  KeyRound,
  LogOut,
  Maximize2,
  MessageSquare,
  Moon,
  RefreshCw,
  Scaling,
  Share2,
  Sun,
  SunMoon,
  UserRoundCog,
} from "lucide-react";
import type {
  CSSProperties,
  KeyboardEvent as ReactKeyboardEvent,
  MouseEvent as ReactMouseEvent,
  ReactNode,
  PointerEvent as ReactPointerEvent,
  SyntheticEvent,
} from "react";
import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useMemo,
  useRef,
  useState,
} from "react";

import {
  type AnnotationSelectorData,
  type AuthIdentity,
  browserAPI,
  createToken,
  type FrontMatter,
  type PreviewAPI,
  revokeToken,
  type SearchResult,
  type TocItem,
} from "./api";
import type { AnnotationCopyFormat } from "./components/annotations/annotation-copy-button";
import {
  type AnnotationCardBounds,
  AnnotationFloatingCard,
  annotationCardHeight,
} from "./components/annotations/annotation-floating-card";
import {
  AnnotationHoverDetail,
  AnnotationHoverList,
} from "./components/annotations/annotation-hover-content";
import { AnnotationPanel } from "./components/annotations/annotation-panel";
import { SelectionToolbar } from "./components/annotations/selection-toolbar";
import { DocumentTree } from "./components/document-tree";
import { FrontMatterSummary } from "./components/frontmatter";
import { HomePage } from "./components/home-page";
import { PreviewContent } from "./components/preview-content";
import { ReaderFooter } from "./components/reader-footer";
import { ReaderNavigation } from "./components/reader-navigation";
import { SearchDialog } from "./components/search-dialog";
import { TokenSettings } from "./components/settings/token-settings";
import {
  ShareDialog,
  ShareSettings,
  type ShareTarget,
} from "./components/share/share-dialogs";
import {
  TableOfContentsPanel,
  TableOfContentsSheet,
  TOCToggle,
} from "./components/table-of-contents";
import { Button } from "./components/ui/button";
import { Input } from "./components/ui/input";
import { ScrollArea } from "./components/ui/scroll-area";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarInset,
  SidebarProvider,
  SidebarRail,
  SidebarTrigger,
} from "./components/ui/sidebar";
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from "./components/ui/tooltip";
import { useAnnotations } from "./hooks/use-annotations";
import { useIsMobile } from "./hooks/use-mobile";
import {
  exportAnnotationsJSON,
  exportAnnotationsMarkdown,
} from "./lib/annotation-export";
import {
  type AnnotationHighlight,
  AnnotationHighlightLayer,
} from "./lib/annotation-highlight";
import { reanchor } from "./lib/annotation-reanchor";
import { TextIndex, type TextPosition } from "./lib/annotation-selector";
import { copyText } from "./lib/clipboard";
import { findHeadingElement } from "./lib/heading";
import { watchReaderScroll } from "./lib/reader-scroll";
import { readScrollPosition, saveScrollPosition } from "./lib/scroll-position";
import {
  activateTabForElement,
  isElementInInactiveTab,
  isTabRadio,
} from "./lib/tabbed";
import {
  type DocumentWidth,
  decodeHeadingHash,
  encodeHeadingHash,
  type Mode,
  markdownURL,
  readRoute,
} from "./model";
import { useHeadingNavigation } from "./use-heading-navigation";
import { useHeadingSpy } from "./use-heading-spy";
import { usePreview } from "./use-preview";
import { useSearch } from "./use-search";

interface AppProps {
  api?: PreviewAPI;
  // Authenticated deployments pass the session identity and a logout
  // handler; the default unauthenticated mode leaves both unset so the
  // toolbar shows no account controls at all.
  authIdentity?: AuthIdentity | null;
  onLogout?: () => void;
  // Whether this deployment exposes annotations and API tokens
  // (authenticated runs).
  annotationsAvailable?: boolean;
  tokensAvailable?: boolean;
  sharesAvailable?: boolean;
}

const modes: Array<{ value: Mode; label: string; icon: typeof Sun }> = [
  { value: "auto", label: "跟随系统", icon: SunMoon },
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
];

const accountThemeOptions = modes.map((option) => ({
  ...option,
  label: option.value === "auto" ? "系统主题" : `${option.label}主题`,
}));

const layoutStorageKey = "m2h.preview.layout";

// Browser-provided client hints identify the operating system without parsing
// the user agent. navigator.platform is retained only as a fallback for older
// browsers that have not implemented User-Agent Client Hints yet.
export function searchShortcutModifier(): "Cmd" | "Ctrl" {
  const clientNavigator = navigator as Navigator & {
    userAgentData?: { platform?: string };
  };
  const platform =
    clientNavigator.userAgentData?.platform ??
    navigator.platform ??
    navigator.userAgent;
  return /mac|iphone|ipad|ipod/i.test(platform) ? "Cmd" : "Ctrl";
}

// The sidebar hint shows the client's usual modifier; the keyboard handler
// below continues to accept both modifiers everywhere.
const searchShortcutHint = `${searchShortcutModifier()}+K`;

// True when this page load came from a reload or a history traversal: the
// reader should return to the exact pixel offset saved for the document (see
// scroll-position.ts — the browser's own restoration was measured NOT to fire
// for this client-rendered shape, so the tab remembers it). Fresh navigations
// (new tab/window on a #hash link, address-bar entry) return false; they land
// on the URL fragment instead.
function cameFromReloadLikeNavigation(): boolean {
  const entry = performance.getEntriesByType("navigation")[0];
  // Duck-typed on purpose: the entry is a PerformanceNavigationTiming in every
  // browser, but jsdom (and tests) supply plain objects.
  if (entry === undefined || !("type" in entry)) {
    return false;
  }
  return (entry as PerformanceNavigationTiming).type !== "navigate";
}

interface AnnotationPopup {
  key: string;
  ids: string[];
  activeId?: string;
  editingId?: string;
  deleting?: boolean;
  deleteError?: string;
  x: number;
  y: number;
  bounds?: AnnotationCardBounds;
  pinned: boolean;
}

function keepAnnotationPopupOpen(popup: AnnotationPopup): boolean {
  return Boolean(
    popup.pinned || popup.editingId || popup.deleting || popup.deleteError,
  );
}

interface StoredLayout {
  sidebarOpen: boolean;
  sidebarWidth: number;
  rightSidebarWidth: number;
}

const rightSidebarMinWidth = 208;
const rightSidebarMaxWidth = 480;

const defaultLayout: StoredLayout = {
  sidebarOpen: true,
  sidebarWidth: 256,
  rightSidebarWidth: 240,
};

const documentWidths: Array<{
  value: DocumentWidth;
  label: string;
  icon: typeof Columns3;
}> = [
  { value: "standard", label: "标准", icon: Columns3 },
  { value: "wide", label: "宽", icon: Scaling },
  { value: "full", label: "全屏", icon: Maximize2 },
];

export function App({
  api,
  authIdentity,
  onLogout,
  annotationsAvailable,
  tokensAvailable,
  sharesAvailable,
}: AppProps) {
  const preview = usePreview(api);
  const [settledHomeRevision, setSettledHomeRevision] = useState(0);
  const homeRefreshing =
    preview.filesLoading ||
    (preview.phase === "home" &&
      preview.homeRequest.forceRefresh &&
      preview.homeRequest.revision !== settledHomeRevision);
  // A single-file scope has nothing to switch between, so the file sidebar and
  // its toolbar trigger stay hidden; directories and multi-root workspaces
  // both offer navigation. Every other control remains shared.
  const navigationAvailable =
    preview.kind === "directory" || preview.kind === "workspace";
  const [initialLayout] = useState(readStoredLayout);
  const [sidebarWidth, setSidebarWidth] = useState(initialLayout.sidebarWidth);
  const [rightSidebarWidth, setRightSidebarWidth] = useState(
    initialLayout.rightSidebarWidth,
  );
  const [sidebarOpen, setSidebarOpen] = useState(initialLayout.sidebarOpen);
  // The mobile Sheet owns its own open state (openMobile) that toggleSidebar
  // flips below 768px; lifting it here lets the tree read one merged fact.
  const [sidebarMobileOpen, setSidebarMobileOpen] = useState(false);
  // The tree's visibility is whichever sidebar host is actually on screen:
  // the mobile Sheet above 768px's persistent column. Deriving it from the
  // desktop flag alone left the active-file reveal dead on mobile whenever
  // the reader had collapsed and persisted the desktop sidebar.
  const isMobileViewport = useIsMobile();
  const sidebarTreeVisible = isMobileViewport ? sidebarMobileOpen : sidebarOpen;
  const [sidebarResizing, setSidebarResizing] = useState(false);
  // The sidebar input filters the file tree by file name and path only — it
  // is file location, served from the listing's topology data alone.
  // Title/description/body search lives in the SearchDialog (Ctrl/Cmd+K)
  // below. The two share nothing but the magnifier icon — different
  // capabilities, different entries.
  const [fileFilterQuery, setFileFilterQuery] = useState("");
  const [searchOpen, setSearchOpen] = useState(false);
  const search = useSearch(api);
  // Search stays global across the whole workspace: results keep their root
  // grouping (two roots may both hold a README.md), and matching a root's
  // name surfaces every document under it.
  const filteredRoots = useMemo(() => {
    const query = fileFilterQuery.trim().toLocaleLowerCase();
    if (query === "") {
      return preview.roots;
    }
    return preview.roots
      .map((root) => {
        if (root.name.toLocaleLowerCase().includes(query)) {
          return root;
        }
        return {
          ...root,
          files: root.files.filter(
            (file) =>
              file.name.toLocaleLowerCase().includes(query) ||
              file.path.toLocaleLowerCase().includes(query),
          ),
        };
      })
      .filter((root) => root.files.length > 0);
  }, [preview.roots, fileFilterQuery]);
  const filteredCount = useMemo(
    () => filteredRoots.reduce((total, root) => total + root.files.length, 0),
    [filteredRoots],
  );
  const multiRoot = preview.roots.length > 1;
  // Transient feedback for document-tree copy actions. One status line at a
  // time, cleared by a timer; role=status announces it politely.
  const [copyStatus, setCopyStatus] = useState<string | null>(null);
  const copyStatusTimer = useRef(0);
  const announceCopyStatus = useCallback((message: string) => {
    setCopyStatus(message);
    window.clearTimeout(copyStatusTimer.current);
    copyStatusTimer.current = window.setTimeout(() => {
      setCopyStatus(null);
    }, 2_000);
  }, []);
  useEffect(
    () => () => {
      window.clearTimeout(copyStatusTimer.current);
    },
    [],
  );
  // Global search shortcut. The workspace-level search is available before
  // any document is selected — single file, directory and multi-root
  // workspaces alike. Since the top-bar search button was removed,
  // Ctrl/Cmd+K is the global search's only entry.
  useLayoutEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key.toLowerCase() === "k" && (event.metaKey || event.ctrlKey)) {
        event.preventDefault();
        setSearchOpen(true);
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, []);
  // The scroll spy follows every heading (H1–H6) so the URL can reflect the
  // reading position even with the TOC panel hidden; the right-hand TOC still
  // lists H2–H4 only (the first H1 is the toolbar title, H5/H6 are too deep for
  // a narrow rail) and derives its highlight from that single source.
  const headings = useMemo<TocItem[]>(
    () => preview.document?.toc ?? [],
    [preview.document],
  );
  const tocItems = useMemo<TocItem[]>(
    () => headings.filter((item) => item.level >= 2 && item.level <= 4),
    [headings],
  );
  const tocVisible = preview.toc && tocItems.length > 0;
  const readerToolbarRef = useRef<HTMLElement>(null);
  const readerMainRef = useRef<HTMLDivElement>(null);
  const activeHeadingID = useHeadingSpy(
    headings,
    readerMainRef,
    preview.phase === "ready",
  );
  // The TOC highlight lags the real position when the active heading is deeper
  // than H4 (or is the document H1): walk back through the full heading list to
  // the nearest H2–H4 ancestor so the rail still marks the enclosing section.
  const activeTocID = useMemo(() => {
    if (activeHeadingID === null) {
      return null;
    }
    const index = headings.findIndex((item) => item.id === activeHeadingID);
    if (index === -1) {
      return null;
    }
    for (let position = index; position >= 0; position -= 1) {
      const candidate = headings[position];
      if (candidate.level >= 2 && candidate.level <= 4) {
        return candidate.id;
      }
    }
    return null;
  }, [headings, activeHeadingID]);
  // One scroll path for the TOC, Markdown fragment links, heading permalinks and
  // deep-link restore: resolve the id inside the Markdown body, scroll to it,
  // and (optionally) push the hash through the single URL funnel.
  const navigateToHeading = useHeadingNavigation(
    readerMainRef,
    preview.replaceHash,
  );
  // Annotations: the document identity the annotation API needs (only
  // present in authenticated runs), the annotation list of the selected
  // document, and the highlight layer rendered onto the live body.
  const annotationsEnabled = annotationsAvailable === true;
  const tokensEnabled = tokensAvailable === true;
  const annotationIdentity = useMemo(() => {
    if (!annotationsEnabled || preview.document === null) {
      return null;
    }
    const { documentRef, documentHash, path } = preview.document;
    if (documentRef === undefined || documentHash === undefined) {
      return null;
    }
    return { path, documentRef, documentHash };
  }, [annotationsEnabled, preview.document]);
  const annotationDocumentKey = JSON.stringify([
    annotationIdentity?.path,
    annotationIdentity?.documentHash,
  ]);
  const currentAnnotationIdentity = useRef(annotationIdentity);
  currentAnnotationIdentity.current = annotationIdentity;
  const annotations = useAnnotations(annotationsEnabled, annotationIdentity);
  const annotationsOpen =
    annotationsEnabled &&
    preview.selectedPath !== null &&
    preview.annotationsOpen;
  const [annotationSelectionVersion, setAnnotationSelectionVersion] =
    useState(0);
  const [tokenSettingsOpen, setTokenSettingsOpen] = useState(false);
  const [shareTarget, setShareTarget] = useState<ShareTarget | null>(null);
  const [shareSettingsOpen, setShareSettingsOpen] = useState(false);
  const sharesEnabled = sharesAvailable === true && authIdentity != null;
  const shareDocument =
    sharesEnabled && preview.phase === "ready" ? preview.document : null;

  const [tocShare, setTocShare] = useState(50);
  const [annotationPopups, setAnnotationPopups] = useState<AnnotationPopup[]>(
    [],
  );
  const hoverTimer = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hoverFrame = useRef<number | null>(null);
  const hoverPoint = useRef({ x: 0, y: 0 });
  const cancelHoverFrame = useCallback(() => {
    if (hoverFrame.current === null) return;
    cancelAnimationFrame(hoverFrame.current);
    hoverFrame.current = null;
  }, []);
  const clearHoverTimer = useCallback(() => {
    if (hoverTimer.current === null) return;
    clearTimeout(hoverTimer.current);
    hoverTimer.current = null;
  }, []);
  function scheduleHoverDismiss() {
    if (hoverTimer.current !== null) return;
    hoverTimer.current = setTimeout(() => {
      hoverTimer.current = null;
      setAnnotationPopups((current) => {
        const retained = current.filter(keepAnnotationPopupOpen);
        return retained.length === current.length ? current : retained;
      });
    }, 250);
  }
  // The human re-anchor flow: the annotation awaiting a new passage, or
  // null. While pending, the selection toolbar serves the re-association
  // instead of creating new annotations.
  const [reanchorPending, setReanchorPending] = useState<{
    id: string;
    quote: string;
  } | null>(null);
  const [annotationStatuses, setAnnotationStatuses] = useState<
    Record<string, "resolved" | "ambiguous" | "orphaned">
  >({});
  const annotationBodyRef = useRef<HTMLElement | null>(null);
  const annotationIndexRef = useRef<TextIndex | null>(null);
  const annotationLayerRef = useRef<AnnotationHighlightLayer | null>(null);
  const annotationPositionsRef = useRef<Map<string, TextPosition>>(new Map());
  const workspaceRoot = preview.selectedPath === null;
  // biome-ignore lint/correctness/useExhaustiveDependencies: Reset the floating card when its document changes.
  useLayoutEffect(() => {
    setAnnotationPopups([]);
    return () => {
      cancelHoverFrame();
      clearHoverTimer();
    };
  }, [annotationIdentity, cancelHoverFrame, clearHoverTimer]);
  useLayoutEffect(() => {
    if (!workspaceRoot) return;
    annotationLayerRef.current?.release();
    annotationBodyRef.current = null;
    annotationIndexRef.current = null;
    annotationPositionsRef.current.clear();
    setAnnotationStatuses({});
  }, [workspaceRoot]);
  // The loaded document's content hash, read through a ref so the rebuild
  // callback keys on list changes only: an annotation whose stored hash
  // equals the loaded hash may trust its stored position, one from an
  // older version must re-earn it through context.
  const annotationDocumentHashRef = useRef<string | null>(null);
  annotationDocumentHashRef.current = annotationIdentity?.documentHash ?? null;
  if (annotationLayerRef.current === null) {
    annotationLayerRef.current = new AnnotationHighlightLayer();
  }
  // rebuildAnnotationLayer re-resolves every annotation against the live
  // body index: exact-match re-anchoring, highlight registration, status
  // computation. It runs after the body settles (onBodyReady) and after
  // every list change; a theme switch never rebuilds it (the body DOM is
  // unchanged), which is exactly the no-flip contract.
  const rebuildAnnotationLayer = useCallback(() => {
    const layer = annotationLayerRef.current;
    if (layer === null) {
      return;
    }
    const index = annotationIndexRef.current;
    if (index === null) {
      layer.release();
      annotationPositionsRef.current.clear();
      setAnnotationStatuses({});
      return;
    }
    const currentHash = annotationDocumentHashRef.current;
    const highlights: AnnotationHighlight[] = [];
    const statuses: Record<string, "resolved" | "ambiguous" | "orphaned"> = {};
    const located = new Map<string, TextPosition>();
    for (const item of annotations.items) {
      const result = reanchor(
        index,
        item.selector,
        item.documentHash === currentHash,
      );
      statuses[item.id] = result.status;
      if (result.position !== null) {
        highlights.push({
          id: item.id,
          color: item.color,
          position: result.position,
        });
        located.set(item.id, result.position);
      }
    }
    annotationPositionsRef.current = located;
    layer.render(index, highlights);
    setAnnotationStatuses(statuses);
  }, [annotations.items]);
  useEffect(() => {
    rebuildAnnotationLayer();
  }, [rebuildAnnotationLayer]);
  useEffect(
    () => () => {
      annotationLayerRef.current?.release();
    },
    [],
  );
  const handleAnnotationBodyReady = useCallback(
    (root: HTMLElement) => {
      annotationBodyRef.current = root;
      annotationIndexRef.current = new TextIndex(root);
      rebuildAnnotationLayer();
    },
    [rebuildAnnotationLayer],
  );
  useEffect(() => {
    if (!annotationsEnabled) return;
    const handleTableSort = (event: Event) => {
      if (
        !(event.target instanceof HTMLTableElement) ||
        !annotationBodyRef.current?.contains(event.target)
      )
        return;
      annotationLayerRef.current?.refreshRanges();
      cancelHoverFrame();
      clearHoverTimer();
      setAnnotationPopups((current) => {
        const retained = current.filter(keepAnnotationPopupOpen);
        return retained.length === current.length ? current : retained;
      });
    };
    // Tablesort's afterSort event reaches ancestors during capture.
    document.addEventListener("afterSort", handleTableSort, true);
    return () =>
      document.removeEventListener("afterSort", handleTableSort, true);
  }, [annotationsEnabled, cancelHoverFrame, clearHoverTimer]);
  const annotationsAt = useCallback((x: number, y: number): string[] => {
    const index = annotationIndexRef.current;
    const positions = annotationPositionsRef.current;
    if (!index || positions.size === 0) return [];
    let offset: number | null = null;
    if (document.caretPositionFromPoint || document.caretRangeFromPoint) {
      const caret = document.caretPositionFromPoint?.(x, y);
      const legacy = caret ? null : document.caretRangeFromPoint?.(x, y);
      const node = caret?.offsetNode ?? legacy?.startContainer;
      const boundary = caret?.offset ?? legacy?.startOffset;
      if (node === undefined || boundary === undefined) return [];
      offset = index.offsetFromBoundary(node, boundary);
      if (offset === null) return [];
    }
    const ids: string[] = [];
    for (const [id, position] of positions) {
      // A caret can snap to either edge of a glyph. Include both endpoints,
      // then use the rectangles to distinguish adjacent highlights.
      if (
        offset !== null &&
        (offset < position.start || offset > position.end)
      ) {
        continue;
      }
      const range = index.rangeFromSpan(position);
      if (
        range &&
        Array.from(range.getClientRects()).some(
          (rect) =>
            x >= rect.left &&
            x <= rect.right &&
            y >= rect.top &&
            y <= rect.bottom,
        )
      ) {
        ids.push(id);
      }
    }
    return ids;
  }, []);
  const transientAnnotationPopup = annotationPopups.find(
    (popup) => !keepAnnotationPopupOpen(popup),
  );
  useEffect(() => {
    if (transientAnnotationPopup === undefined) return;
    const { key } = transientAnnotationPopup;
    const rememberPointer = ({ clientX, clientY }: MouseEvent) => {
      hoverPoint.current = { x: clientX, y: clientY };
    };
    // Scrolling moves the highlight without necessarily firing mousemove.
    const stopWatchingScroll = watchReaderScroll(annotationBodyRef, (body) => {
      const { x, y } = hoverPoint.current;
      const element = document.elementFromPoint(x, y);
      if (
        element?.closest(".annotation-hover-card") ||
        (body.contains(element) && JSON.stringify(annotationsAt(x, y)) === key)
      ) {
        return;
      }
      setAnnotationPopups((current) =>
        current.filter(
          (popup) => popup.key !== key || keepAnnotationPopupOpen(popup),
        ),
      );
    });
    window.addEventListener("mousemove", rememberPointer, {
      capture: true,
      passive: true,
    });
    return () => {
      window.removeEventListener("mousemove", rememberPointer, true);
      stopWatchingScroll();
    };
  }, [annotationsAt, transientAnnotationPopup]);
  const visibleAnnotationPopups = useMemo(
    () =>
      annotationPopups.flatMap((popup) => {
        const items = annotations.items
          .filter((item) => popup.ids.includes(item.id))
          // The API lists in insertion order when timestamps tie.
          .reverse()
          .sort((a, b) => b.createdAt - a.createdAt);
        return items.length > 0 ? [{ popup, items }] : [];
      }),
    [annotations.items, annotationPopups],
  );
  const latestAnnotationPopup = visibleAnnotationPopups.at(-1);
  const hoveredAnnotationId =
    latestAnnotationPopup?.popup.activeId ??
    latestAnnotationPopup?.items[0]?.id ??
    null;
  useEffect(() => {
    annotationLayerRef.current?.emphasize(
      annotations.selectedId,
      hoveredAnnotationId,
    );
  }, [annotations.selectedId, hoveredAnnotationId]);

  function showHoveredAnnotations(
    ids: string[],
    x: number,
    y: number,
    pinned = false,
    bounds?: AnnotationCardBounds,
  ) {
    clearHoverTimer();
    const key = JSON.stringify(ids);
    setAnnotationPopups((current) => {
      const existing = current.find((popup) => popup.key === key);
      const others = current.filter(
        (popup) => popup.key !== key && keepAnnotationPopupOpen(popup),
      );
      if (
        existing &&
        others.length + 1 === current.length &&
        (!pinned || existing.pinned)
      ) {
        return current;
      }
      return [
        ...others,
        existing
          ? { ...existing, pinned: existing.pinned || pinned }
          : { key, ids, x, y, pinned, bounds },
      ];
    });
  }

  function updateAnnotationPopup(
    key: string,
    changes: Partial<AnnotationPopup>,
  ) {
    setAnnotationPopups((current) =>
      current.map((popup) =>
        popup.key === key ? { ...popup, ...changes } : popup,
      ),
    );
  }

  function closeAnnotationPopup(key: string) {
    setAnnotationPopups((current) =>
      current.filter((popup) => popup.key !== key),
    );
  }

  const locateAnnotation = useCallback(
    (id: string) => {
      const position = annotationPositionsRef.current.get(id);
      const index = annotationIndexRef.current;
      if (position === undefined || index === null) {
        return;
      }
      const range = index.rangeFromSpan(position);
      const element =
        range?.startContainer.parentElement ?? annotationBodyRef.current;
      element?.scrollIntoView({ behavior: "smooth", block: "center" });
      annotations.select(id);
      window.setTimeout(() => annotations.select(null), 2400);
    },
    [annotations],
  );
  // Each copy issues a short-lived credential scoped to these exact bytes.
  const copyAnnotations = useCallback(
    async (format: AnnotationCopyFormat, ids?: string[]): Promise<boolean> => {
      if (
        annotationIdentity === null ||
        preview.document === null ||
        !tokensEnabled
      )
        return false;
      const selected = annotations.items.filter(
        (item) => ids === undefined || ids.includes(item.id),
      );
      if (
        selected.length === 0 ||
        (ids !== undefined && selected.length !== ids.length)
      )
        return false;
      let tokenID: string | undefined;
      try {
        const token = await createToken({
          kind: "temporary",
          name: "文档评审",
          expiresInDays: 30 / 1440,
          scope: {
            permissions: ["document:read"],
            resource: {
              type: "document",
              rootId: annotationIdentity.documentRef.rootId,
              path: annotationIdentity.documentRef.path,
            },
          },
          documentHash: annotationIdentity.documentHash,
        });
        tokenID = token.id;
        if (!token.secret) throw new Error("temporary token is missing");
        const current = currentAnnotationIdentity.current;
        if (
          !current ||
          current.path !== annotationIdentity.path ||
          current.documentHash !== annotationIdentity.documentHash ||
          current.documentRef.rootId !== annotationIdentity.documentRef.rootId
        ) {
          throw new Error("document changed during copy");
        }
        const doc = {
          title: preview.document.title,
          path: preview.document.path,
          ref: annotationIdentity.documentRef,
          hash: annotationIdentity.documentHash,
        };
        const entries = selected.map((item) => ({
          id: item.id,
          documentPath: doc.path,
          documentHash: item.documentHash,
          authorName: item.authorName,
          comment: item.comment,
          color: item.color,
          selector: item.selector,
          status: annotationStatuses[item.id] ?? "resolved",
          createdAt: item.createdAt,
          updatedAt: item.updatedAt,
        }));
        const credentials = {
          url: `${window.location.origin}${markdownURL(doc.path)}`,
          token: token.secret,
          expiresInMinutes: 30,
        };
        const packet =
          format === "markdown"
            ? exportAnnotationsMarkdown(doc, entries, credentials)
            : exportAnnotationsJSON(doc, entries, credentials);
        if (!(await copyText(packet)))
          throw new Error("clipboard write failed");
        return true;
      } catch {
        if (tokenID) {
          try {
            await revokeToken(tokenID);
          } catch {
            /* Expiry remains enforced if revocation fails. */
          }
        }
        return false;
      }
    },
    [
      annotationIdentity,
      preview.document,
      tokensEnabled,
      annotations.items,
      annotationStatuses,
    ],
  );

  const annotationExportDocument = useMemo(() => {
    if (preview.document === null || annotationIdentity === null) {
      return null;
    }
    return {
      title: preview.document.title,
      path: preview.document.path,
      ref: annotationIdentity.documentRef,
      hash: annotationIdentity.documentHash,
    };
  }, [preview.document, annotationIdentity]);

  // The re-anchor flow: a document switch mid-flow orphans the pending
  // re-association (the annotation belongs to the previous document), so
  // the identity change cancels it — state adjusted during render, not in
  // an effect, so the pending flow can never paint onto a new document.
  const reanchorIdentityRef = useRef(annotationIdentity);
  if (reanchorIdentityRef.current !== annotationIdentity) {
    reanchorIdentityRef.current = annotationIdentity;
    if (reanchorPending !== null) {
      setReanchorPending(null);
    }
  }
  const startReanchor = useCallback(
    (id: string) => {
      setReanchorPending((current) => {
        if (current !== null && current.id === id) {
          return null; // clicking again toggles the flow off
        }
        const quote =
          annotations.items.find((item) => item.id === id)?.selector.quote
            .exact ?? "";
        return { id, quote };
      });
    },
    [annotations.items],
  );
  const confirmReanchor = useCallback(
    async (selector: AnnotationSelectorData): Promise<boolean> => {
      if (reanchorPending === null) {
        return false;
      }
      const saved = await annotations.reanchor(reanchorPending.id, selector);
      if (saved) {
        setReanchorPending(null);
      }
      return saved;
    },
    [reanchorPending, annotations],
  );
  // Search-result activation mirrors Markdown-link navigation exactly: a hit
  // in the already-open document scrolls in place (never refetching, so
  // Mermaid/图片 etc. are not re-rendered), a hit in another document
  // goes through the normal select path with the section hash encoded by the
  // one canonical encoder. A metadata-only hit on the current document just
  // clears the fragment and returns to the top.
  const pendingSearchRef = useRef<SearchResult | null>(null);
  const handleSearchSelect = (result: SearchResult) => {
    if (result.path === preview.selectedPath) {
      if (result.heading !== undefined || result.tabId !== undefined) {
        navigateToHeading(result.heading?.id ?? "", {
          tabId: result.tabId,
          behavior: "smooth",
          updateURL: true,
        });
      } else {
        preview.replaceHash(null);
        window.scrollTo({ top: 0, behavior: "smooth" });
      }
      return;
    }
    pendingSearchRef.current = result.tabId === undefined ? null : result;
    void preview.select(
      result.path,
      result.heading !== undefined ? encodeHeadingHash(result.heading.id) : "",
    );
  };
  // Landing: position the reader once a document commits.
  // - Initial load after a reload/traversal: restore the pixel offset saved
  //   for this document (the browser's native restoration does not fire for a
  //   client-rendered body — measured, see scroll-position.ts), falling back
  //   to the URL fragment when nothing was saved.
  // - Fresh navigation (a #hash link opened in a new tab/window): the fragment
  //   wins. sessionStorage is deliberately ignored there — a tab opened from a
  //   link inherits a copy of it in Chromium and would jump to wherever the
  //   opener was instead of the shared section.
  // - In-app document switch: a carried fragment wins, else the offset saved
  //   earlier in this tab, else the viewport stays put.
  // - A body hot-swap of the same document never repositions: the window keeps
  //   its offset and CSS scroll anchoring absorbs the reflow.
  const reloadLikeNavigationRef = useRef<boolean | null>(null);
  if (reloadLikeNavigationRef.current === null) {
    reloadLikeNavigationRef.current = cameFromReloadLikeNavigation();
  }
  const documentPath = preview.document?.path ?? null;
  const landedPathRef = useRef<string | null>(null);
  useEffect(() => {
    if (preview.phase === "home") {
      landedPathRef.current = null;
      pendingSearchRef.current = null;
      if (window.scrollY !== 0) window.scrollTo(0, 0);
    }
  }, [preview.phase]);
  useEffect(() => {
    if (preview.phase !== "ready" || documentPath === null) {
      return;
    }
    const pending = pendingSearchRef.current;
    pendingSearchRef.current = null;
    if (pending !== null && pending.path === documentPath) {
      landedPathRef.current = documentPath;
      navigateToHeading(pending.heading?.id ?? "", {
        tabId: pending.tabId,
        behavior: "auto",
      });
      return;
    }
    const isFirstLoad = landedPathRef.current === null;
    const documentSwitched = landedPathRef.current !== documentPath;
    landedPathRef.current = documentPath;
    if (!documentSwitched) {
      return;
    }
    const fragment = decodeHeadingHash(window.location.hash);
    if (isFirstLoad && !reloadLikeNavigationRef.current && fragment !== "") {
      navigateToHeading(fragment, { behavior: "auto", updateURL: false });
      return;
    }
    // Restore visibility before a saved pixel offset; default selection may
    // otherwise hide the heading carried by a reload's hash.
    const target =
      readerMainRef.current === null
        ? null
        : findHeadingElement(readerMainRef.current, fragment);
    if (target !== null) activateTabForElement(target);
    const saved = readScrollPosition(documentPath);
    if (saved !== null) {
      window.scrollTo(0, saved);
      return;
    }
    if (fragment !== "") {
      navigateToHeading(fragment, { behavior: "auto", updateURL: false });
    }
  }, [documentPath, preview.phase, navigateToHeading]);

  useEffect(() => {
    const root = readerMainRef.current;
    if (root === null || preview.phase !== "ready") return;
    const changed = (event: Event) => {
      if (!isTabRadio(event.target)) return;
      const heading = findHeadingElement(
        root,
        decodeHeadingHash(window.location.hash),
      );
      if (heading !== null && isElementInInactiveTab(heading))
        preview.replaceHash(null);
    };
    root.addEventListener("change", changed);
    return () => root.removeEventListener("change", changed);
  }, [preview.phase, preview.replaceHash]);

  // Persist the window's scroll offset per document — rAF-throttled so a long
  // scroll writes at most once per frame, and flushed synchronously on
  // pagehide so a scroll immediately followed by a reload keeps the last
  // offset. This is what the landing effect above restores after a reload;
  // the browser's own restoration never fires for this shape.
  useEffect(() => {
    if (preview.phase !== "ready" || documentPath === null) {
      return;
    }
    let frame = 0;
    const persist = () => {
      frame = 0;
      saveScrollPosition(documentPath, window.scrollY);
    };
    const handleScroll = () => {
      if (frame !== 0) {
        return;
      }
      frame = requestAnimationFrame(persist);
    };
    // pagehide (not beforeunload) fires for reloads and navigations without
    // opting out of the back/forward cache.
    const handlePageHide = () => {
      if (frame !== 0) {
        cancelAnimationFrame(frame);
        persist();
      }
    };
    window.addEventListener("scroll", handleScroll, { passive: true });
    window.addEventListener("pagehide", handlePageHide);
    return () => {
      if (frame !== 0) {
        cancelAnimationFrame(frame);
      }
      window.removeEventListener("scroll", handleScroll);
      window.removeEventListener("pagehide", handlePageHide);
    };
  }, [preview.phase, documentPath]);

  // Reflect the reading position into the URL as the user scrolls. replaceState
  // (never push) keeps the back stack clean across the many headings a scroll
  // passes, and no-op when the position already matches the URL. The spy only
  // reacts to real scroll events, so the native restore settles first and the
  // hash then reflects wherever the viewport landed.
  // The funnel stays silent until the spy is live (phase ready): before that
  // there is no reading position to reflect, and a mount-time write would
  // strip a deep link's fragment before the landing effect has consumed it.
  // A ref rather than a dep — the effect must key on position changes only,
  // not re-run when the phase flips.
  const spyReadyRef = useRef(false);
  spyReadyRef.current = preview.phase === "ready";
  useEffect(() => {
    if (!spyReadyRef.current) {
      return;
    }
    const currentID = decodeHeadingHash(window.location.hash);
    const nextID = activeHeadingID ?? "";
    if (nextID === currentID) {
      return;
    }
    preview.replaceHash(activeHeadingID);
  }, [activeHeadingID, preview.replaceHash]);

  useEffect(() => {
    try {
      window.localStorage.setItem(
        layoutStorageKey,
        JSON.stringify({ sidebarOpen, sidebarWidth, rightSidebarWidth }),
      );
    } catch {
      // Storage can be unavailable in private browsing; the layout still works.
    }
  }, [sidebarOpen, sidebarWidth, rightSidebarWidth]);

  const selectMarkdownTarget = (targetElement: EventTarget | null): boolean => {
    if (!(targetElement instanceof Element)) {
      return false;
    }
    const anchor = targetElement.closest("a");
    if (!(anchor instanceof HTMLAnchorElement)) {
      return false;
    }
    const target = new URL(anchor.href, window.location.href);
    if (target.origin !== window.location.origin) {
      return false;
    }
    const route = readRoute(target.pathname, target.search, target.hash);
    const currentPath = preview.selectedPath;
    // A link whose target is the already-open document — most commonly a bare
    // "#id" fragment, or an explicit path back to the same file — must never
    // trigger a reload: scroll to the heading in place and let the hash update
    // flow through the URL funnel. Intercepting it (rather than returning false)
    // also stops the browser from reloading the page on a same-URL navigation.
    if (route.path !== null && route.path === currentPath) {
      const id = decodeHeadingHash(route.hash);
      if (id !== "") {
        navigateToHeading(id, { behavior: "smooth", updateURL: true });
      }
      return true;
    }
    // Otherwise only intercept genuine cross-document links under /doc/.
    if (
      route.path === null ||
      !preview.files.some((file) => file.path === route.path)
    ) {
      return false;
    }
    void preview.select(route.path, route.hash);
    return true;
  };

  const handleMarkdownClick = (event: ReactMouseEvent<HTMLElement>) => {
    if (selectMarkdownTarget(event.target)) {
      event.preventDefault();
    }
  };

  const handleMarkdownKeyDown = (event: ReactKeyboardEvent<HTMLElement>) => {
    if (event.key === "Enter" && selectMarkdownTarget(event.target)) {
      event.preventDefault();
    }
  };

  const handleAssetError = (event: SyntheticEvent<HTMLElement>) => {
    if (event.target instanceof HTMLImageElement) {
      const image = event.target;
      // The placeholder collapsing a failed image in place lives in the body
      // DOM (render-rich-content.ts). Its own load failure arrives here too —
      // it only means the app itself is broken, not that a second document
      // asset failed, so it never reports. The reported source is the one
      // that actually failed, which after the swap no longer equals "src".
      if (image.dataset.m2hFallback === "true") {
        return;
      }
      const source =
        image.dataset.m2hOriginalSrc ?? image.getAttribute("src") ?? "";
      preview.reportVisualError(
        source === "" ? "图片加载失败。" : `图片加载失败：${source}`,
      );
    }
  };

  return (
    <TooltipProvider delay={350}>
      <SidebarProvider
        className="app-shell"
        style={{ "--sidebar-width": `${sidebarWidth}px` } as CSSProperties}
        open={sidebarOpen}
        onOpenChange={setSidebarOpen}
        openMobile={sidebarMobileOpen}
        onOpenMobileChange={setSidebarMobileOpen}
      >
        {navigationAvailable ? (
          <Sidebar collapsible="offcanvas" resizing={sidebarResizing}>
            <SidebarHeader className="pb-0">
              <div className="sidebar-search">
                <Funnel aria-hidden="true" />
                <Input
                  type="search"
                  value={fileFilterQuery}
                  aria-label="按文件名或路径筛选文件"
                  placeholder={`筛选文件名或路径（${searchShortcutHint}全文搜索）`}
                  onChange={(event) => setFileFilterQuery(event.target.value)}
                />
              </div>
              <div className="flex h-8 shrink-0 items-center justify-between pr-2">
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        aria-label="文档首页"
                        aria-current={
                          preview.phase === "home" ? "page" : undefined
                        }
                        className="aria-[current=page]:bg-muted"
                        onClick={() => {
                          preview.goHome();
                          setSidebarMobileOpen(false);
                        }}
                      >
                        <House aria-hidden="true" />
                      </Button>
                    }
                  />
                  <TooltipContent side="bottom">文档首页</TooltipContent>
                </Tooltip>
                <span className="text-xs tabular-nums text-sidebar-foreground/60">
                  <span aria-hidden="true">{filteredCount}</span>
                  <span className="sr-only">
                    {filteredCount} 个 Markdown 文件
                  </span>
                </span>
              </div>
            </SidebarHeader>
            {/* SidebarContent keeps overflow-clip on desktop so it never
             * becomes a second scroll owner: unlike overflow-hidden it
             * hard-clips without establishing a scroll container at all, so
             * the ScrollArea viewport below stays the sidebar's only scroll
             * geometry — sticky rows, the active-file reveal and wheel input
             * all act on it, and paint is cut at one deterministic boundary.
             * Mobile drops the clip inside the fixed Sheet, where the
             * viewport is already the tree's only scroll container. The one
             * mobile-specific behavior left is the Sheet's initial focus,
             * see the Sidebar mobile branch in ui/sidebar.tsx. */}
            <SidebarContent className="min-w-0 overflow-visible md:overflow-clip">
              <ScrollArea
                className="tree-scroll"
                scrollbarVisibility="scrolling"
                contentProps={{
                  // The tree is vertical-only: shrink Base UI Content's default
                  // minWidth: fit-content back to the viewport width so a long
                  // filename can never grow scrollWidth past clientWidth.
                  style: { minWidth: 0, width: "100%" },
                }}
              >
                <SidebarGroup>
                  <SidebarGroupContent>
                    {filteredCount > 0 ? (
                      multiRoot ? (
                        filteredRoots.map((root) => (
                          <DocumentTree
                            key={root.id}
                            files={root.files}
                            metadata={preview.metadata}
                            onRequestMetadata={(path) =>
                              void preview.loadFileMetadata(path)
                            }
                            rootBase={root.id}
                            rootLabel={root.name}
                            onCopyStatus={announceCopyStatus}
                            searching={fileFilterQuery.trim() !== ""}
                            selectedPath={preview.selectedPath}
                            visible={sidebarTreeVisible}
                            onSelect={(path) => void preview.select(path)}
                          />
                        ))
                      ) : (
                        <DocumentTree
                          files={filteredRoots[0]?.files ?? []}
                          metadata={preview.metadata}
                          onRequestMetadata={(path) =>
                            void preview.loadFileMetadata(path)
                          }
                          onCopyStatus={announceCopyStatus}
                          searching={fileFilterQuery.trim() !== ""}
                          selectedPath={preview.selectedPath}
                          visible={sidebarTreeVisible}
                          onSelect={(path) => void preview.select(path)}
                        />
                      )
                    ) : (
                      <p className="tree-placeholder">
                        {/* The sidebar owns its own loading/failure state: a
                         * file-list error never occupies the reader's error
                         * panel — an open document stays on screen. */}
                        {preview.filesLoading
                          ? "正在加载文件…"
                          : preview.filesError !== null
                            ? "无法读取文件列表"
                            : fileFilterQuery.trim() !== ""
                              ? "没有匹配的文档"
                              : "目录中没有 Markdown 文件"}
                      </p>
                    )}
                  </SidebarGroupContent>
                </SidebarGroup>
              </ScrollArea>
            </SidebarContent>
            <SidebarResizeHandle
              width={sidebarWidth}
              onResize={setSidebarWidth}
              onResizeStart={() => setSidebarResizing(true)}
              onResizeEnd={() => setSidebarResizing(false)}
            />
          </Sidebar>
        ) : null}

        <SidebarInset
          className="reader-inset"
          style={
            { "--reader-toc-width": `${rightSidebarWidth}px` } as CSSProperties
          }
        >
          <header className="reader-toolbar" ref={readerToolbarRef}>
            {navigationAvailable ? (
              <Tooltip>
                <TooltipTrigger
                  render={
                    <SidebarTrigger
                      className="toolbar-control"
                      aria-label="切换文件导航"
                    />
                  }
                />
                <TooltipContent side="bottom">
                  {sidebarOpen ? "收起文件导航" : "展开文件导航"}
                </TooltipContent>
              </Tooltip>
            ) : null}
            <DocumentTitle
              title={
                preview.phase === "home"
                  ? "文档首页"
                  : (preview.document?.title ?? null)
              }
              frontmatter={preview.document?.frontmatter ?? null}
            />
            <div className="toolbar-actions">
              {preview.phase === "home" || preview.phase === "empty" ? (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        className="toolbar-control"
                        aria-label="刷新文档首页"
                        aria-busy={homeRefreshing}
                        disabled={homeRefreshing}
                        onClick={() => void preview.refresh()}
                      />
                    }
                  >
                    <RefreshCw
                      aria-hidden="true"
                      className={homeRefreshing ? "is-spinning" : undefined}
                    />
                  </TooltipTrigger>
                  <TooltipContent side="bottom">刷新文档首页</TooltipContent>
                </Tooltip>
              ) : null}
              {shareDocument?.documentRef !== undefined ? (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        className="toolbar-control"
                        aria-label="分享文档"
                        onClick={() => {
                          if (shareDocument.documentRef !== undefined) {
                            setShareTarget({
                              title: shareDocument.title,
                              documentRef: shareDocument.documentRef,
                            });
                          }
                        }}
                      />
                    }
                  >
                    <Share2 aria-hidden="true" />
                  </TooltipTrigger>
                  <TooltipContent side="bottom">分享文档</TooltipContent>
                </Tooltip>
              ) : null}
              {authIdentity !== null &&
              authIdentity !== undefined &&
              onLogout !== undefined ? (
                <Menu.Root modal={false}>
                  <Tooltip>
                    <Menu.Trigger
                      render={
                        <TooltipTrigger
                          render={
                            <Button
                              variant="ghost"
                              size="icon"
                              className="toolbar-control"
                              aria-label="设置"
                            />
                          }
                        />
                      }
                    >
                      <UserRoundCog aria-hidden="true" />
                    </Menu.Trigger>
                    <TooltipContent side="bottom">设置</TooltipContent>
                  </Tooltip>
                  <Menu.Portal>
                    <Menu.Positioner
                      align="end"
                      sideOffset={6}
                      className="theme-menu-positioner"
                    >
                      <Menu.Popup className="theme-menu-popup settings-menu-popup">
                        <PreferenceSubmenu label="页面主题" icon={SunMoon}>
                          <PreferenceMenuOptions
                            options={accountThemeOptions}
                            value={preview.mode}
                            onChange={preview.setMode}
                          />
                        </PreferenceSubmenu>
                        <PreferenceSubmenu label="正文宽度" icon={Columns3}>
                          <PreferenceMenuOptions
                            options={documentWidths}
                            value={preview.width}
                            onChange={preview.setWidth}
                          />
                        </PreferenceSubmenu>
                        {tokensEnabled ? (
                          <Menu.Item
                            className="theme-menu-item"
                            onClick={() => setTokenSettingsOpen(true)}
                          >
                            <KeyRound aria-hidden="true" />
                            凭据管理
                          </Menu.Item>
                        ) : null}
                        {sharesEnabled ? (
                          <Menu.Item
                            className="theme-menu-item"
                            onClick={() => setShareSettingsOpen(true)}
                          >
                            <Share2 aria-hidden="true" />
                            分享管理
                          </Menu.Item>
                        ) : null}
                        <Menu.Item
                          className="theme-menu-item"
                          onClick={() => void onLogout()}
                        >
                          <LogOut aria-hidden="true" />
                          退出登录（{authIdentity.displayName}）
                        </Menu.Item>
                      </Menu.Popup>
                    </Menu.Positioner>
                  </Menu.Portal>
                </Menu.Root>
              ) : (
                <>
                  <DocumentWidthMenu
                    width={preview.width}
                    onChange={preview.setWidth}
                  />
                  <ThemeMenu mode={preview.mode} onChange={preview.setMode} />
                </>
              )}
              {sharesEnabled && shareTarget !== null ? (
                <ShareDialog
                  target={shareTarget}
                  onClose={() => setShareTarget(null)}
                />
              ) : null}
              {sharesEnabled ? (
                <ShareSettings
                  open={shareSettingsOpen}
                  onOpenChange={setShareSettingsOpen}
                />
              ) : null}
              {tokensEnabled ? (
                <TokenSettings
                  open={tokenSettingsOpen}
                  onOpenChange={setTokenSettingsOpen}
                />
              ) : null}
              {annotationsEnabled && !workspaceRoot ? (
                <Tooltip>
                  <TooltipTrigger
                    render={
                      <Button
                        variant="ghost"
                        size="icon"
                        className="toolbar-control"
                        aria-pressed={annotationsOpen}
                        aria-label={
                          annotationsOpen ? "收起标注面板" : "打开标注面板"
                        }
                        onClick={() =>
                          preview.setAnnotationsOpen(!annotationsOpen)
                        }
                      >
                        <MessageSquare aria-hidden="true" />
                      </Button>
                    }
                  />
                  <TooltipContent side="bottom">
                    {annotationsOpen ? "收起标注面板" : "打开标注面板"}
                  </TooltipContent>
                </Tooltip>
              ) : null}
              {preview.phase !== "home" ? (
                <TOCToggle
                  enabled={preview.toc}
                  available={tocItems.length > 0}
                  onChange={preview.setTOC}
                />
              ) : null}
              {/* Narrow screens hide the rail and its toggle above; the sheet
               * trigger takes over as the transient outline UI there. */}
              {tocItems.length > 0 ? (
                <TableOfContentsSheet
                  items={tocItems}
                  activeID={activeTocID}
                  onNavigate={(id) =>
                    navigateToHeading(id, {
                      behavior: "smooth",
                      updateURL: true,
                    })
                  }
                />
              ) : null}
            </div>
          </header>

          {/* biome-ignore lint/a11y/noStaticElementInteractions: Pointer tracking reads text highlight geometry; existing text remains selectable. */}
          <div
            className="reader-main"
            ref={readerMainRef}
            onMouseMove={(event) => {
              if (
                visibleAnnotationPopups.some(
                  ({ popup }) => !popup.pinned && popup.editingId,
                ) ||
                !(event.target instanceof Node) ||
                !annotationBodyRef.current?.contains(event.target)
              ) {
                cancelHoverFrame();
                return;
              }
              hoverPoint.current = { x: event.clientX, y: event.clientY };
              if (hoverFrame.current !== null) return;
              hoverFrame.current = requestAnimationFrame(() => {
                hoverFrame.current = null;
                const { x, y } = hoverPoint.current;
                const ids = annotationsAt(x, y);
                if (ids.length > 0) {
                  showHoveredAnnotations(ids, x, y);
                } else {
                  scheduleHoverDismiss();
                }
              });
            }}
            onMouseLeave={() => {
              cancelHoverFrame();
              scheduleHoverDismiss();
            }}
            onClickCapture={(event) => {
              cancelHoverFrame();
              if (
                !(event.target instanceof Node) ||
                !annotationBodyRef.current?.contains(event.target) ||
                window.getSelection()?.toString()
              )
                return;
              const ids = annotationsAt(event.clientX, event.clientY);
              if (ids.length > 1) {
                showHoveredAnnotations(ids, event.clientX, event.clientY, true);
              } else if (ids[0] !== undefined) {
                preview.setAnnotationsOpen(true);
                annotations.select(ids[0]);
                setAnnotationSelectionVersion((value) => value + 1);
              }
            }}
          >
            <div className={`reader-canvas reader-canvas-${preview.width}`}>
              {preview.visualError !== null ? (
                <div className="asset-warning" role="status">
                  <ImageOff aria-hidden="true" />
                  <span>{preview.visualError}</span>
                </div>
              ) : null}
              {preview.phase === "home" ? (
                <HomePage
                  getRecent={(api ?? browserAPI).getRecent}
                  revision={preview.homeRequest.revision}
                  forceRefresh={preview.homeRequest.forceRefresh}
                  onRecentSettled={setSettledHomeRevision}
                  fileCount={preview.files.length}
                  roots={preview.roots}
                  options={preview}
                  onSelect={(path) => void preview.select(path)}
                />
              ) : (
                <PreviewContent
                  phase={preview.phase}
                  error={preview.error}
                  html={preview.document?.html ?? null}
                  frontmatter={preview.document?.frontmatter ?? null}
                  resolvedMode={preview.resolvedMode}
                  onRetry={() => void preview.retry()}
                  onClick={handleMarkdownClick}
                  onKeyDown={handleMarkdownKeyDown}
                  onErrorCapture={handleAssetError}
                  onVisualError={preview.reportVisualError}
                  onBodyReady={
                    annotationsEnabled ? handleAnnotationBodyReady : undefined
                  }
                />
              )}
              {/* The attribution belongs to a successfully opened document
               * page, not the app shell: loading, home, not-found and
               * error states stay footer-less. */}
              {preview.phase === "ready" && preview.document !== null ? (
                <ReaderFooter version={preview.version} />
              ) : null}
            </div>
            {/* The desktop rail stays mounted whenever the document has a TOC
             * (only the sheet handles narrow screens); `visible` drives its
             * expand/collapse animation, mirroring the sidebar's persistent
             * gap + container pair instead of mounting and unmounting. */}
            <div
              className="reader-rail-slot"
              data-open={
                (preview.toc && tocItems.length > 0) || annotationsOpen
              }
            >
              <div
                className="reader-rail"
                style={{ "--toc-share": `${tocShare}%` } as CSSProperties}
                data-toc-open={preview.toc && tocItems.length > 0}
                data-annotations-open={annotationsOpen}
              >
                {!isMobileViewport &&
                ((preview.toc && tocItems.length > 0) || annotationsOpen) ? (
                  <RightSidebarResizeHandle
                    width={rightSidebarWidth}
                    onResize={setRightSidebarWidth}
                  />
                ) : null}
                {tocItems.length > 0 ? (
                  <TableOfContentsPanel
                    visible={preview.toc}
                    items={tocItems}
                    activeID={activeTocID}
                    onNavigate={(id) =>
                      navigateToHeading(id, {
                        behavior: "smooth",
                        updateURL: true,
                      })
                    }
                  />
                ) : null}
                {preview.toc && tocItems.length > 0 && annotationsOpen ? (
                  <>
                    {/* biome-ignore lint/a11y/useSemanticElements: This focusable separator supports pointer and keyboard resizing. */}
                    <div
                      role="separator"
                      aria-label="调整目录与标注高度"
                      aria-orientation="horizontal"
                      aria-valuenow={tocShare}
                      aria-valuemin={20}
                      aria-valuemax={80}
                      tabIndex={0}
                      className="reader-rail-resize sidebar-rail-line"
                      onPointerDown={(event) =>
                        event.currentTarget.setPointerCapture(event.pointerId)
                      }
                      onPointerMove={(event) => {
                        if (
                          !event.currentTarget.hasPointerCapture(
                            event.pointerId,
                          )
                        )
                          return;
                        const rect =
                          event.currentTarget.parentElement?.getBoundingClientRect();
                        if (rect)
                          setTocShare(
                            Math.max(
                              20,
                              Math.min(
                                80,
                                Math.round(
                                  ((event.clientY - rect.top) / rect.height) *
                                    100,
                                ),
                              ),
                            ),
                          );
                      }}
                      onKeyDown={(event) => {
                        if (
                          event.key === "ArrowUp" ||
                          event.key === "ArrowDown"
                        ) {
                          event.preventDefault();
                          setTocShare((value) =>
                            Math.max(
                              20,
                              Math.min(
                                80,
                                value + (event.key === "ArrowUp" ? -5 : 5),
                              ),
                            ),
                          );
                        }
                      }}
                    />
                  </>
                ) : null}
                {annotationsEnabled && !workspaceRoot ? (
                  <AnnotationPanel
                    key={annotationDocumentKey}
                    open={annotationsOpen}
                    onOpenChange={preview.setAnnotationsOpen}
                    items={annotations.items}
                    statuses={annotationStatuses}
                    loading={annotations.loading}
                    error={annotations.error}
                    selectedId={annotations.selectedId}
                    selectionVersion={annotationSelectionVersion}
                    document={annotationExportDocument}
                    onLocate={locateAnnotation}
                    onDelete={annotations.remove}
                    onUpdateAnnotation={annotations.update}
                    onReanchor={startReanchor}
                    onCopy={tokensEnabled ? copyAnnotations : undefined}
                  />
                ) : null}
              </div>
            </div>
          </div>
          {!workspaceRoot &&
            visibleAnnotationPopups.map(({ popup, items }) => {
              const item =
                items.length === 1
                  ? items[0]
                  : items.find(({ id }) => id === popup.activeId);
              const editing = item !== undefined && item.id === popup.editingId;
              const selectItem = (activeId?: string) =>
                updateAnnotationPopup(popup.key, {
                  activeId,
                  editingId: undefined,
                  deleteError: undefined,
                  pinned: true,
                });

              return (
                <AnnotationFloatingCard
                  key={popup.key}
                  left={popup.bounds?.left ?? popup.x}
                  top={
                    popup.bounds?.top ??
                    (popup.ids.length > 1 &&
                    popup.y + annotationCardHeight + 24 > window.innerHeight
                      ? popup.y - annotationCardHeight - 16
                      : popup.y + 16)
                  }
                  initialSize={popup.bounds}
                  closeDisabled={popup.deleting}
                  pinned={popup.pinned}
                  onPinnedChange={(pinned) => {
                    clearHoverTimer();
                    updateAnnotationPopup(popup.key, { pinned });
                  }}
                  onAdjust={() => {
                    if (!popup.pinned)
                      updateAnnotationPopup(popup.key, { pinned: true });
                  }}
                  onEnter={() => {
                    cancelHoverFrame();
                    if (!popup.pinned) clearHoverTimer();
                  }}
                  onLeave={() => {
                    if (!keepAnnotationPopupOpen(popup))
                      closeAnnotationPopup(popup.key);
                  }}
                  onClose={() => closeAnnotationPopup(popup.key)}
                >
                  {item ? (
                    <AnnotationHoverDetail
                      key={item.id}
                      item={item}
                      editing={editing}
                      onEditingChange={(editing) =>
                        updateAnnotationPopup(popup.key, {
                          editingId: editing ? item.id : undefined,
                        })
                      }
                      onBack={items.length > 1 ? () => selectItem() : undefined}
                      onCopy={
                        tokensEnabled
                          ? (format) => copyAnnotations(format, [item.id])
                          : undefined
                      }
                      onSave={(comment, color) =>
                        annotations.update(item.id, comment, color)
                      }
                      deleting={popup.deleting ?? false}
                      deleteError={popup.deleteError}
                      onDelete={() => {
                        updateAnnotationPopup(popup.key, {
                          deleting: true,
                          deleteError: undefined,
                        });
                        void annotations
                          .remove(item.id)
                          .then(({ failures }) => {
                            const failure = failures[0];
                            if (!failure && items.length === 1) {
                              closeAnnotationPopup(popup.key);
                              return;
                            }
                            updateAnnotationPopup(popup.key, {
                              deleting: false,
                              deleteError: failure?.message,
                            });
                          });
                      }}
                    />
                  ) : (
                    <AnnotationHoverList items={items} onSelect={selectItem} />
                  )}
                </AnnotationFloatingCard>
              );
            })}
          {annotationsEnabled ? (
            <>
              {reanchorPending !== null ? (
                <div className="copy-status" role="status" aria-live="polite">
                  请在正文中选中要关联的新文本（按 Esc 取消）
                  <Button
                    variant="ghost"
                    size="xs"
                    aria-label="取消重新关联"
                    onClick={() => setReanchorPending(null)}
                  >
                    取消
                  </Button>
                </div>
              ) : null}
              <SelectionToolbar
                key={annotationDocumentKey}
                disabled={annotationIdentity === null}
                bodyRef={annotationBodyRef}
                readerToolbarRef={readerToolbarRef}
                indexRef={annotationIndexRef}
                onAnnotate={(selector, comment, color, pinnedBounds) =>
                  annotations.create(
                    selector,
                    comment,
                    color,
                    pinnedBounds &&
                      ((id) =>
                        showHoveredAnnotations(
                          [id],
                          pinnedBounds.left,
                          pinnedBounds.top,
                          true,
                          pinnedBounds,
                        )),
                  )
                }
                reanchorPending={reanchorPending}
                onReanchor={confirmReanchor}
                onReanchorCancel={() => setReanchorPending(null)}
              />
            </>
          ) : null}
          {/* Floating edge jumps for the window-scrolled reader; one instance
           * for both single-file and directory previews. When the TOC rail is
           * up, the buttons shift left of it (see .reader-navigation CSS). */}
          {preview.phase !== "home" || !isMobileViewport ? (
            <ReaderNavigation tocVisible={tocVisible || annotationsOpen} />
          ) : null}
          {copyStatus !== null ? (
            <div className="copy-status" role="status" aria-live="polite">
              {copyStatus}
            </div>
          ) : null}
          {/* The workspace-level full-text search overlay. Always mounted so
           * the shortcut and toolbar trigger work from any reading state;
           * Base UI renders nothing while closed. */}
          <SearchDialog
            open={searchOpen}
            onOpenChange={setSearchOpen}
            search={search}
            onSelect={handleSearchSelect}
          />
        </SidebarInset>
      </SidebarProvider>
    </TooltipProvider>
  );
}

function readStoredLayout(): StoredLayout {
  try {
    const value: unknown = JSON.parse(
      window.localStorage.getItem(layoutStorageKey) ?? "null",
    );
    if (typeof value !== "object" || value === null) {
      return defaultLayout;
    }
    const candidate = value as Record<string, unknown>;
    const sidebarWidth = candidate.sidebarWidth;
    const rightSidebarWidth = candidate.rightSidebarWidth;
    return {
      sidebarOpen:
        typeof candidate.sidebarOpen === "boolean"
          ? candidate.sidebarOpen
          : defaultLayout.sidebarOpen,
      sidebarWidth:
        typeof sidebarWidth === "number" && Number.isFinite(sidebarWidth)
          ? Math.min(480, Math.max(208, sidebarWidth))
          : defaultLayout.sidebarWidth,
      rightSidebarWidth:
        typeof rightSidebarWidth === "number" &&
        Number.isFinite(rightSidebarWidth)
          ? clampRightSidebarWidth(rightSidebarWidth)
          : defaultLayout.rightSidebarWidth,
    };
  } catch {
    return defaultLayout;
  }
}

function DocumentTitle({
  title,
  frontmatter,
}: {
  title: string | null;
  frontmatter: FrontMatter | null;
}) {
  return (
    <section className="document-title" aria-label="当前文档标题">
      <div className="document-title-text">{title ?? "未选择文档"}</div>
      <FrontMatterSummary frontmatter={frontmatter} />
    </section>
  );
}

function SidebarResizeHandle({
  width,
  onResize,
  onResizeStart,
  onResizeEnd,
}: {
  width: number;
  onResize(width: number): void;
  onResizeStart(): void;
  onResizeEnd(): void;
}) {
  const handlePointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = width;
    const wrapper = event.currentTarget.closest(
      '[data-slot="sidebar-wrapper"]',
    );
    let nextWidth = startWidth;
    const handlePointerMove = (moveEvent: PointerEvent) => {
      nextWidth = Math.min(
        480,
        Math.max(208, startWidth + moveEvent.clientX - startX),
      );
      if (wrapper instanceof HTMLElement) {
        wrapper.style.setProperty("--sidebar-width", `${nextWidth}px`);
      }
    };
    const handlePointerEnd = () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerEnd);
      window.removeEventListener("pointercancel", handlePointerEnd);
      onResize(nextWidth);
      onResizeEnd();
    };
    onResizeStart();
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerEnd);
    window.addEventListener("pointercancel", handlePointerEnd);
  };

  return (
    <SidebarRail
      className="sidebar-resize-handle"
      aria-label="调整侧边栏宽度"
      title="拖动调整侧边栏宽度"
      onPointerDown={handlePointerDown}
      onClick={(event) => event.preventDefault()}
    />
  );
}

function clampRightSidebarWidth(width: number): number {
  return Math.min(
    rightSidebarMaxWidth,
    Math.max(rightSidebarMinWidth, Math.round(width)),
  );
}

function RightSidebarResizeHandle({
  width,
  onResize,
}: {
  width: number;
  onResize(width: number): void;
}) {
  const drag = useRef<{
    inset: HTMLElement;
    pointerId: number;
    startX: number;
    startWidth: number;
    nextWidth: number;
  } | null>(null);

  useLayoutEffect(
    () => () => {
      const current = drag.current;
      if (current === null) return;
      current.inset.style.setProperty(
        "--reader-toc-width",
        `${current.startWidth}px`,
      );
      delete current.inset.dataset.railResizing;
      drag.current = null;
    },
    [],
  );

  return (
    // biome-ignore lint/a11y/useSemanticElements: This focusable separator adjusts the width with pointer and keyboard input.
    <div
      role="separator"
      aria-label="调整右侧边栏宽度"
      aria-orientation="vertical"
      aria-valuemin={rightSidebarMinWidth}
      aria-valuemax={rightSidebarMaxWidth}
      aria-valuenow={width}
      tabIndex={0}
      className="reader-rail-width-resize sidebar-rail-line"
      title="拖动或按方向键调整右侧边栏宽度"
      onPointerDown={(event) => {
        if (event.button !== 0 || drag.current !== null) return;
        const inset = event.currentTarget.closest<HTMLElement>(".reader-inset");
        if (inset === null) return;
        event.preventDefault();
        event.currentTarget.setPointerCapture(event.pointerId);
        inset.dataset.railResizing = "true";
        drag.current = {
          inset,
          pointerId: event.pointerId,
          startX: event.clientX,
          startWidth: width,
          nextWidth: width,
        };
      }}
      onPointerMove={(event) => {
        const current = drag.current;
        if (current?.pointerId !== event.pointerId) return;
        current.nextWidth = clampRightSidebarWidth(
          current.startWidth + current.startX - event.clientX,
        );
        current.inset.style.setProperty(
          "--reader-toc-width",
          `${current.nextWidth}px`,
        );
      }}
      onLostPointerCapture={(event) => {
        const current = drag.current;
        if (current?.pointerId !== event.pointerId) return;
        delete current.inset.dataset.railResizing;
        drag.current = null;
        onResize(current.nextWidth);
      }}
      onKeyDown={(event) => {
        if (drag.current !== null) return;
        const nextWidths: Record<string, number> = {
          ArrowLeft: width + 16,
          ArrowRight: width - 16,
          Home: rightSidebarMinWidth,
          End: rightSidebarMaxWidth,
        };
        const next = nextWidths[event.key];
        if (typeof next !== "number") return;
        event.preventDefault();
        onResize(clampRightSidebarWidth(next));
      }}
    />
  );
}

function PreferenceSubmenu({
  label,
  icon: Icon,
  children,
}: {
  label: string;
  icon: typeof Sun;
  children: ReactNode;
}) {
  return (
    <Menu.SubmenuRoot>
      <Menu.SubmenuTrigger className="theme-menu-item">
        <Icon aria-hidden="true" />
        <span>{label}</span>
        <ChevronRight aria-hidden="true" />
      </Menu.SubmenuTrigger>
      <Menu.Portal>
        <Menu.Positioner
          className="theme-menu-positioner"
          side="left"
          align="start"
          sideOffset={4}
        >
          <Menu.Popup
            className="theme-menu-popup settings-submenu-popup"
            aria-label={label}
          >
            {children}
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.SubmenuRoot>
  );
}

function PreferenceMenuOptions<Value extends string>({
  options,
  value,
  onChange,
}: {
  options: { value: Value; label: string; icon: typeof Sun }[];
  value: Value;
  onChange(value: Value): void;
}) {
  return (
    <Menu.RadioGroup
      value={value}
      onValueChange={(nextValue) => {
        const option = options.find((item) => item.value === nextValue);
        if (option) onChange(option.value);
      }}
    >
      {options.map(({ value, label, icon: Icon }) => (
        <Menu.RadioItem
          key={value}
          value={value}
          className="theme-menu-item"
          closeOnClick
        >
          <Icon aria-hidden="true" />
          <span>{label}</span>
          <Menu.RadioItemIndicator className="theme-menu-indicator">
            <Check aria-hidden="true" />
          </Menu.RadioItemIndicator>
        </Menu.RadioItem>
      ))}
    </Menu.RadioGroup>
  );
}

export function DocumentWidthMenu({
  width,
  onChange,
}: {
  width: DocumentWidth;
  onChange(width: DocumentWidth): void;
}) {
  const current =
    documentWidths.find((item) => item.value === width) ?? documentWidths[0];
  const CurrentIcon = current.icon;
  return (
    // Non-modal: toolbar dropdowns must not trap focus or lock document scroll.
    <Menu.Root modal={false}>
      <Tooltip>
        <Menu.Trigger
          render={
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  className="document-width-control"
                  aria-label={`文档宽度：${current.label}`}
                />
              }
            />
          }
        >
          <CurrentIcon aria-hidden="true" />
        </Menu.Trigger>
        <TooltipContent side="bottom">调整文档宽度</TooltipContent>
      </Tooltip>
      <Menu.Portal>
        <Menu.Positioner
          className="theme-menu-positioner"
          align="end"
          sideOffset={6}
        >
          <Menu.Popup className="theme-menu-popup">
            <Menu.Group>
              <Menu.GroupLabel className="theme-menu-label">
                文档宽度
              </Menu.GroupLabel>
              <PreferenceMenuOptions
                options={documentWidths}
                value={width}
                onChange={onChange}
              />
            </Menu.Group>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}

export function ThemeMenu({
  mode,
  onChange,
}: {
  mode: Mode;
  onChange(mode: Mode): void;
}) {
  const currentLabel = modes.find((item) => item.value === mode)?.label ?? mode;
  const CurrentIcon =
    modes.find((item) => item.value === mode)?.icon ?? SunMoon;

  return (
    // Non-modal: toolbar dropdowns must not trap focus or lock document scroll.
    <Menu.Root modal={false}>
      <Tooltip>
        <Menu.Trigger
          render={
            <TooltipTrigger
              render={
                <Button
                  variant="ghost"
                  size="icon-sm"
                  aria-label={`显示主题：${currentLabel}`}
                />
              }
            />
          }
        >
          <CurrentIcon aria-hidden="true" />
        </Menu.Trigger>
        <TooltipContent side="bottom">切换显示主题</TooltipContent>
      </Tooltip>
      <Menu.Portal>
        <Menu.Positioner
          className="theme-menu-positioner"
          align="end"
          sideOffset={6}
        >
          <Menu.Popup className="theme-menu-popup">
            <Menu.Group>
              <Menu.GroupLabel className="theme-menu-label">
                显示主题
              </Menu.GroupLabel>
              <PreferenceMenuOptions
                options={modes}
                value={mode}
                onChange={onChange}
              />
            </Menu.Group>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}
