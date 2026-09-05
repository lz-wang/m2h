import { Menu } from "@base-ui/react/menu";
import {
  Check,
  Columns3,
  FileQuestion,
  FileText,
  Funnel,
  ImageOff,
  Inbox,
  LoaderCircle,
  Maximize2,
  Moon,
  RefreshCw,
  Scaling,
  Sun,
  SunMoon,
  TriangleAlert,
} from "lucide-react";
import type {
  CSSProperties,
  KeyboardEvent as ReactKeyboardEvent,
  MouseEvent as ReactMouseEvent,
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

import type { FrontMatter, PreviewAPI, SearchResult, TocItem } from "./api";
import { DocumentLightbox } from "./components/document-lightbox";
import { DocumentTree } from "./components/document-tree";
import { FrontMatterPanel, FrontMatterSummary } from "./components/frontmatter";
import { ReaderNavigation } from "./components/reader-navigation";
import { SearchDialog } from "./components/search-dialog";
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
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
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
import {
  collectLightboxState,
  type LightboxState,
} from "./lib/document-lightbox";
import { enhanceDocumentLinks } from "./lib/document-links";
import {
  finalizeVegaLiteViews,
  renderRichContent,
  rerenderThemeSensitiveContent,
} from "./lib/render-rich-content";
import { readScrollPosition, saveScrollPosition } from "./lib/scroll-position";
import {
  type DocumentWidth,
  decodeHeadingHash,
  encodeHeadingHash,
  type Mode,
  type ResolvedMode,
  readRoute,
} from "./model";
import { useHeadingNavigation } from "./use-heading-navigation";
import { useHeadingSpy } from "./use-heading-spy";
import { usePreview } from "./use-preview";
import { useSearch } from "./use-search";

interface AppProps {
  api?: PreviewAPI;
}

const modes: Array<{ value: Mode; label: string; icon: typeof Sun }> = [
  { value: "auto", label: "跟随系统", icon: SunMoon },
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
];

const layoutStorageKey = "m2h.preview.layout";
const repositoryURL = "https://github.com/lz-wang/m2h";
const releaseVersionPattern = /^\d+\.\d+\.\d+$/;

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

interface StoredLayout {
  sidebarOpen: boolean;
  sidebarWidth: number;
}

const defaultLayout: StoredLayout = {
  sidebarOpen: true,
  sidebarWidth: 256,
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

export function App({ api }: AppProps) {
  const preview = usePreview(api);
  // A single-file scope has nothing to switch between, so the file sidebar and
  // its toolbar trigger stay hidden; directories and multi-root workspaces
  // both offer navigation. Every other control remains shared.
  const navigationAvailable =
    preview.kind === "directory" || preview.kind === "workspace";
  const [initialLayout] = useState(readStoredLayout);
  const [sidebarWidth, setSidebarWidth] = useState(initialLayout.sidebarWidth);
  const [sidebarOpen, setSidebarOpen] = useState(initialLayout.sidebarOpen);
  const [sidebarResizing, setSidebarResizing] = useState(false);
  // The sidebar input filters the file tree by name/title/path/description;
  // full-text search lives in the SearchDialog below. The two share nothing
  // but the magnifier icon — different capabilities, different entries.
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
              file.title.toLocaleLowerCase().includes(query) ||
              file.path.toLocaleLowerCase().includes(query) ||
              (file.description?.toLocaleLowerCase().includes(query) ?? false),
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
  const loading =
    preview.phase === "loading-files" || preview.phase === "loading-document";
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
  // workspaces alike. Toolbar and shortcut are two entries to the same
  // dialog; mobile has no keyboard, so the toolbar button is not optional.
  useEffect(() => {
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
  // Search-result activation mirrors Markdown-link navigation exactly: a hit
  // in the already-open document scrolls in place (never refetching, so
  // Mermaid/Vega/图片 etc. are not re-rendered), a hit in another document
  // goes through the normal select path with the section hash encoded by the
  // one canonical encoder. A metadata-only hit on the current document just
  // clears the fragment and returns to the top.
  const handleSearchSelect = (result: SearchResult) => {
    if (result.path === preview.selectedPath) {
      if (result.heading !== undefined) {
        navigateToHeading(result.heading.id, {
          behavior: "smooth",
          updateURL: true,
        });
      } else {
        preview.replaceHash(null);
        window.scrollTo({ top: 0, behavior: "smooth" });
      }
      return;
    }
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
    if (preview.phase !== "ready" || documentPath === null) {
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
    const saved = readScrollPosition(documentPath);
    if (saved !== null) {
      window.scrollTo(0, saved);
      return;
    }
    if (fragment !== "") {
      navigateToHeading(fragment, { behavior: "auto", updateURL: false });
    }
  }, [documentPath, preview.phase, navigateToHeading]);

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
  useEffect(() => {
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
        JSON.stringify({ sidebarOpen, sidebarWidth }),
      );
    } catch {
      // Storage can be unavailable in private browsing; the layout still works.
    }
  }, [sidebarOpen, sidebarWidth]);

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
      !target.pathname.startsWith("/doc/") ||
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
      >
        {navigationAvailable ? (
          <Sidebar collapsible="offcanvas" resizing={sidebarResizing}>
            <SidebarHeader>
              <div className="sidebar-search">
                <Funnel aria-hidden="true" />
                <Input
                  type="search"
                  value={fileFilterQuery}
                  aria-label="筛选文件"
                  placeholder={`筛选文件（${searchShortcutHint}全文搜索）`}
                  onChange={(event) => setFileFilterQuery(event.target.value)}
                />
              </div>
            </SidebarHeader>
            {/* Desktop keeps overflow-clip so SidebarContent never becomes a
             * second scroll owner: unlike overflow-hidden it hard-clips
             * without establishing a scroll container at all, so the
             * ScrollArea viewport below stays the sidebar's only scroll
             * geometry — sticky rows, the active-file reveal and wheel input
             * all act on it, and paint is cut at one deterministic boundary.
             * Mobile drops the clip: inside the fixed Sheet, the clipped
             * ancestor around the viewport left the first touch scroll dead
             * until some interaction rebuilt the scrolling layer, while the
             * viewport there is already the tree's only scroll container. */}
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
                  <SidebarGroupLabel className="justify-between">
                    <span>Files</span>
                    <span className="text-xs tabular-nums text-sidebar-foreground/60">
                      <span aria-hidden="true">{filteredCount}</span>
                      <span className="sr-only">
                        {filteredCount} 个 Markdown 文件
                      </span>
                    </span>
                  </SidebarGroupLabel>
                  <SidebarGroupContent>
                    {filteredCount > 0 ? (
                      multiRoot ? (
                        filteredRoots.map((root) => (
                          <DocumentTree
                            key={root.id}
                            files={root.files}
                            rootBase={root.id}
                            rootLabel={root.name}
                            onCopyStatus={announceCopyStatus}
                            searching={fileFilterQuery.trim() !== ""}
                            selectedPath={preview.selectedPath}
                            visible={sidebarOpen}
                            onSelect={(path) => void preview.select(path)}
                          />
                        ))
                      ) : (
                        <DocumentTree
                          files={filteredRoots[0]?.files ?? []}
                          onCopyStatus={announceCopyStatus}
                          searching={fileFilterQuery.trim() !== ""}
                          selectedPath={preview.selectedPath}
                          visible={sidebarOpen}
                          onSelect={(path) => void preview.select(path)}
                        />
                      )
                    ) : (
                      <p className="tree-placeholder">
                        {loading
                          ? "正在加载文件…"
                          : fileFilterQuery.trim() !== ""
                            ? "没有匹配的文档"
                            : "目录中没有 Markdown 文件"}
                      </p>
                    )}
                  </SidebarGroupContent>
                </SidebarGroup>
              </ScrollArea>
            </SidebarContent>
            <ProjectFooter version={preview.version} />
            <SidebarResizeHandle
              width={sidebarWidth}
              onResize={setSidebarWidth}
              onResizeStart={() => setSidebarResizing(true)}
              onResizeEnd={() => setSidebarResizing(false)}
            />
          </Sidebar>
        ) : null}

        <SidebarInset className="reader-inset">
          <header className="reader-toolbar">
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
              title={preview.document?.title ?? null}
              frontmatter={preview.document?.frontmatter ?? null}
            />
            <div className="toolbar-actions">
              <DocumentWidthMenu
                width={preview.width}
                onChange={preview.setWidth}
              />
              <ThemeMenu mode={preview.mode} onChange={preview.setMode} />
              <TOCToggle
                enabled={preview.toc}
                available={tocItems.length > 0}
                onChange={preview.setTOC}
              />
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

          <div className="reader-main" ref={readerMainRef}>
            <div className={`reader-canvas reader-canvas-${preview.width}`}>
              {preview.visualError !== null ? (
                <div className="asset-warning" role="status">
                  <ImageOff aria-hidden="true" />
                  <span>{preview.visualError}</span>
                </div>
              ) : null}
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
              />
            </div>
            {/* The desktop rail stays mounted whenever the document has a TOC
             * (only the sheet handles narrow screens); `visible` drives its
             * expand/collapse animation, mirroring the sidebar's persistent
             * gap + container pair instead of mounting and unmounting. */}
            {tocItems.length > 0 ? (
              <TableOfContentsPanel
                visible={preview.toc}
                items={tocItems}
                activeID={activeTocID}
                onNavigate={(id) =>
                  navigateToHeading(id, { behavior: "smooth", updateURL: true })
                }
              />
            ) : null}
          </div>
          {/* Floating edge jumps for the window-scrolled reader; one instance
           * for both single-file and directory previews. When the TOC rail is
           * up, the buttons shift left of it (see .reader-navigation CSS). */}
          <ReaderNavigation tocVisible={tocVisible} />
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

function ProjectFooter({ version }: { version: string }) {
  const releaseVersion = releaseVersionPattern.test(version);
  const versionLabel = releaseVersion ? `v${version}` : version;
  const versionURL = releaseVersion
    ? `${repositoryURL}/releases/tag/${versionLabel}`
    : `${repositoryURL}/releases`;

  return (
    <SidebarFooter className="project-footer flex-row shrink-0 flex-nowrap items-center justify-start gap-1 px-3 py-2">
      <a
        href={repositoryURL}
        target="_blank"
        rel="noreferrer"
        aria-label="在新页面打开 m2h GitHub 仓库"
        title="GitHub"
        className="group flex size-8 items-center justify-center rounded-md text-sidebar-foreground/60 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
      >
        <img
          src="/ui/github-invertocat.svg"
          alt=""
          aria-hidden="true"
          className="h-4 w-auto opacity-60 transition-opacity group-hover:opacity-100 dark:invert"
        />
        <span className="sr-only">在新页面打开 m2h GitHub 仓库</span>
      </a>
      {version !== "" ? (
        <a
          href={versionURL}
          target="_blank"
          rel="noreferrer"
          aria-label={`在新页面打开 m2h ${versionLabel} 发布信息`}
          title="查看发布信息"
          className="project-footer-version shrink-0 rounded-md px-2 py-1 font-mono text-[11px] whitespace-nowrap tabular-nums text-sidebar-foreground/55 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        >
          {versionLabel}
        </a>
      ) : null}
    </SidebarFooter>
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
    return {
      sidebarOpen:
        typeof candidate.sidebarOpen === "boolean"
          ? candidate.sidebarOpen
          : defaultLayout.sidebarOpen,
      sidebarWidth:
        typeof sidebarWidth === "number" && Number.isFinite(sidebarWidth)
          ? Math.min(480, Math.max(208, sidebarWidth))
          : defaultLayout.sidebarWidth,
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

function DocumentWidthMenu({
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
              <Menu.RadioGroup
                value={width}
                onValueChange={(value) => {
                  if (
                    value === "standard" ||
                    value === "wide" ||
                    value === "full"
                  )
                    onChange(value);
                }}
              >
                {documentWidths.map(({ value, label, icon: Icon }) => (
                  <Menu.RadioItem
                    key={value}
                    value={value}
                    className="theme-menu-item"
                  >
                    <Icon aria-hidden="true" />
                    <span>{label}</span>
                    <Menu.RadioItemIndicator className="theme-menu-indicator">
                      <Check aria-hidden="true" />
                    </Menu.RadioItemIndicator>
                  </Menu.RadioItem>
                ))}
              </Menu.RadioGroup>
            </Menu.Group>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}

function ThemeMenu({
  mode,
  onChange,
}: {
  mode: Mode;
  onChange(mode: Mode): void;
}) {
  const currentLabel = modes.find((item) => item.value === mode)?.label ?? mode;
  const CurrentIcon =
    modes.find((item) => item.value === mode)?.icon ?? SunMoon;

  const handleChange = (value: string) => {
    if (value === "light" || value === "dark" || value === "auto") {
      onChange(value);
    }
  };

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
              <Menu.RadioGroup value={mode} onValueChange={handleChange}>
                {modes.map(({ value, label, icon: Icon }) => (
                  <Menu.RadioItem
                    key={value}
                    value={value}
                    className="theme-menu-item"
                  >
                    <Icon aria-hidden="true" />
                    <span>{label}</span>
                    <Menu.RadioItemIndicator className="theme-menu-indicator">
                      <Check aria-hidden="true" />
                    </Menu.RadioItemIndicator>
                  </Menu.RadioItem>
                ))}
              </Menu.RadioGroup>
            </Menu.Group>
          </Menu.Popup>
        </Menu.Positioner>
      </Menu.Portal>
    </Menu.Root>
  );
}

interface PreviewContentProps {
  phase: ReturnType<typeof usePreview>["phase"];
  error: string | null;
  html: string | null;
  frontmatter: FrontMatter | null;
  resolvedMode: ResolvedMode;
  onRetry(): void;
  onClick(event: ReactMouseEvent<HTMLElement>): void;
  onKeyDown(event: ReactKeyboardEvent<HTMLElement>): void;
  onErrorCapture(event: SyntheticEvent<HTMLElement>): void;
  onVisualError(message: string): void;
}

function PreviewContent({
  phase,
  error,
  html,
  frontmatter,
  resolvedMode,
  onRetry,
  onClick,
  onKeyDown,
  onErrorCapture,
  onVisualError,
}: PreviewContentProps) {
  const contentRef = useRef<HTMLElement>(null);
  // Body generation: bumped on every body swap and teardown, never by a
  // theme switch. It is the freshness token handed to the renderers below —
  // a slow render that outlives its document must not apply after the body
  // has moved on — while a theme switch queues behind the initial render
  // instead of invalidating it (see the theme effect below).
  const bodyGenerationRef = useRef(0);
  // The initial enhancement of the current body, awaited by every theme
  // repaint: Vega-Lite chart containers only exist once the initial render
  // has created them, so repainting while the runtime is still downloading
  // would find no targets — and a toggle that invalidated the initial render
  // instead would leave the raw fenced blocks behind for good.
  const initialRenderRef = useRef<Promise<void> | null>(null);
  // Theme repaints of one body run strictly one after another here. vegaEmbed
  // mutates its target container during the await, so two concurrent repaints
  // interleave their DOM writes nondeterministically — a freshness check
  // after the embed resolves cannot un-write what a stale embed already did.
  const themeQueueRef = useRef<Promise<void>>(Promise.resolve());
  // The lightbox snapshot and its open flag are deliberately separate states:
  // closing only flips the flag, the popup stays mounted through its exit
  // transition, and the snapshot is dropped once the dialog reports the
  // transition finished. A whole-state drop (below, on a body swap) skips the
  // exit animation on purpose — the body the snapshots belong to is gone.
  const [lightbox, setLightbox] = useState<LightboxState | null>(null);
  const [lightboxOpen, setLightboxOpen] = useState(false);
  // The resolved theme is read through a ref so the body render effect can
  // depend on [html, phase] only and a theme switch never rebuilds the
  // article. renderedModeRef records the newest theme the theme effect has
  // claimed — it dedupes effect reruns and lets a superseded queued repaint
  // skip — while paintedModeRef records the theme the current body's rich
  // visuals were last painted in, so a repaint whose result is already on
  // screen never runs.
  const resolvedModeRef = useRef<ResolvedMode>(resolvedMode);
  const renderedModeRef = useRef<ResolvedMode | null>(null);
  const paintedModeRef = useRef<ResolvedMode | null>(null);
  resolvedModeRef.current = resolvedMode;

  // React owns the <article> container; the Markdown body DOM is owned by
  // the rich-content renderer. Writing innerHTML in a layout effect runs
  // before paint so there is no empty-body flash.
  //
  // The body effect keys on `phase` and `html` only. Only the "ready" phase
  // renders the <article>; re-entering "ready" (e.g. refreshing the current
  // document, where `html` is the same string) remounts the node, so without
  // `phase` the fresh <article> would render empty. `resolvedMode` stays out
  // of the deps on purpose: rebuilding the article on a theme switch would
  // discard DOM identity, in-body focus, and the KaTeX/copy-button
  // enhancement. The resolved theme is read through `resolvedModeRef` so the
  // body is still painted in the current theme without re-running on toggles.
  // Width and sidebar are likewise excluded.
  //
  // The separate theme effect below regenerates only the theme-sensitive
  // rich visuals (Mermaid diagrams and Vega-Lite charts), whose colors are
  // baked into their SVG at render time. The body generation guard pairs
  // with the renderers' freshness checks so a slow render that outlives its
  // document is not applied after the body has moved on; cleanup invalidates
  // the in-flight render and finalizes Vega views.
  useLayoutEffect(() => {
    if (phase !== "ready") {
      return;
    }
    const root = contentRef.current;
    if (root === null || html === null) {
      return;
    }
    // The body below is wholesale replaced, so any open Lightbox is dropped
    // immediately (no exit animation): its snapshots belong to the outgoing
    // document.
    setLightboxOpen(false);
    setLightbox(null);
    const generation = ++bodyGenerationRef.current;
    const mode = resolvedModeRef.current;
    root.innerHTML = html;
    // Link policy first, before the rich-content enhancements: external
    // links must open in a new tab even when a later renderer bails.
    enhanceDocumentLinks(root);
    renderedModeRef.current = mode;
    paintedModeRef.current = mode;
    initialRenderRef.current = renderRichContent(root, {
      mode,
      isCurrent: () => bodyGenerationRef.current === generation,
      onVisualError,
    });
    return () => {
      bodyGenerationRef.current++;
      initialRenderRef.current = null;
      // Vega views hold timers and document-level listeners that only
      // finalize detaches; this cleanup runs while the old body DOM is still
      // in place, right before the next paint replaces it wholesale.
      finalizeVegaLiteViews(root);
    };
  }, [html, phase, onVisualError]);

  // A theme switch repaints only the theme-sensitive rich visuals (Mermaid
  // diagrams and Vega-Lite charts), leaving the article DOM, KaTeX, copy
  // buttons, and any in-body focus untouched. On the initial render the body
  // effect above has already painted the current theme, so renderedModeRef
  // matches resolvedMode and this effect is a no-op; it only does work on a
  // subsequent light/dark toggle.
  //
  // The repaint is queued, never run inline. Each queued repaint waits for
  // the initial enhancement of the current body — a toggle while a runtime
  // is still downloading must not look for chart containers that do not
  // exist yet — and for any earlier repaint, so one container never has two
  // embeds in flight. A queued repaint is skipped when a newer toggle has
  // claimed a different theme (rapid light → dark → light ends with one
  // light repaint at most), when its result is already on screen, or when
  // the body it belonged to is gone.
  useEffect(() => {
    if (phase !== "ready") {
      return;
    }
    if (renderedModeRef.current === resolvedMode) {
      return;
    }
    const root = contentRef.current;
    if (root === null) {
      return;
    }
    renderedModeRef.current = resolvedMode;
    const mode = resolvedMode;
    const generation = bodyGenerationRef.current;
    const repaint = async () => {
      if (bodyGenerationRef.current !== generation) {
        return;
      }
      // The initial render of this body owns the container swap; only once
      // it has settled (or failed without containers) is there something to
      // repaint. A load failure is swallowed here so one broken download
      // cannot poison the queue; the renderer already reported it.
      const initial = initialRenderRef.current;
      if (initial !== null) {
        await initial.catch(() => {});
        if (bodyGenerationRef.current !== generation) {
          return;
        }
      }
      if (renderedModeRef.current !== mode || paintedModeRef.current === mode) {
        return;
      }
      await rerenderThemeSensitiveContent(root, {
        mode,
        isCurrent: () => {
          return bodyGenerationRef.current === generation;
        },
        onVisualError,
      });
      if (bodyGenerationRef.current === generation) {
        paintedModeRef.current = mode;
      }
    };
    // The tail catch keeps the queue alive after a failed repaint instead of
    // wedging every later toggle behind a rejected promise.
    themeQueueRef.current = themeQueueRef.current.then(repaint).catch(() => {});
  }, [phase, resolvedMode, onVisualError]);

  // The magnifier triggers are injected into the article DOM by
  // render-rich-content.ts, outside React's tree. Rather than wiring
  // imperative callbacks into that layer, clicks bubble to this single React
  // handler: a trigger press is intercepted before the Markdown-link logic
  // ever sees it, and everything else falls through unchanged.
  const handleContentClick = (event: ReactMouseEvent<HTMLElement>) => {
    const target = event.target instanceof Element ? event.target : null;
    const candidate = target?.closest(".m2h-lightbox-trigger");
    if (
      candidate instanceof HTMLButtonElement &&
      contentRef.current?.contains(candidate) === true
    ) {
      event.preventDefault();
      event.stopPropagation();
      // Resolve the pressed trigger's own item — an image or a rendered
      // Mermaid container — through its frame, then index it against the
      // body's current DOM order at click time. No index is baked into the
      // DOM: a sortable table moves <tr> rows after the triggers were
      // injected, so a recorded position could address another item than the
      // one under this trigger.
      const root = contentRef.current;
      // Image frames and rich-visual frames (Mermaid, and later Vega-Lite)
      // share the trigger button; the shared class keeps this lookup from
      // enumerating every visual engine.
      const frame = candidate.closest(
        ".m2h-image-frame, .m2h-rich-visual-frame",
      );
      const selectedItem = frame?.querySelector<HTMLElement>(
        '[data-m2h-lightbox-item="true"]',
      );
      if (root !== null && selectedItem != null) {
        const state = collectLightboxState(root, selectedItem);
        if (state !== null) {
          setLightbox(state);
          setLightboxOpen(true);
        }
      }
      return;
    }
    onClick(event);
  };

  if (phase === "loading-files" || phase === "loading-document") {
    return (
      <section className="state-panel" aria-live="polite" aria-busy="true">
        <LoaderCircle className="is-spinning" aria-hidden="true" />
        <p>{phase === "loading-files" ? "正在加载文件" : "正在加载文档"}</p>
      </section>
    );
  }
  // Deliberately not the "empty" panel: documents exist, the reader just has
  // not picked one yet. Conflating the two would make "no Markdown" and
  // "nothing selected" indistinguishable again.
  if (phase === "unselected") {
    return (
      <section className="state-panel" role="status">
        <FileQuestion aria-hidden="true" />
        <p>请选择要查看的文件</p>
      </section>
    );
  }
  if (phase === "empty") {
    return (
      <section className="state-panel" aria-live="polite">
        <Inbox aria-hidden="true" />
        <p>目录中没有 Markdown 文件</p>
        <span>添加 .md 或 .markdown 文件后重新打开预览。</span>
      </section>
    );
  }
  if (phase === "not-found") {
    return (
      <section className="state-panel" role="status">
        <FileQuestion aria-hidden="true" />
        <p>您浏览的文档不存在或已被删除</p>
      </section>
    );
  }
  if (phase === "error") {
    return (
      <section className="state-panel error-panel" role="alert">
        <TriangleAlert aria-hidden="true" />
        <p>{error ?? "发生未知错误"}</p>
        <Button variant="outline" onClick={onRetry}>
          <RefreshCw aria-hidden="true" />
          重新加载
        </Button>
      </section>
    );
  }
  if (html === null) {
    return (
      <section className="state-panel" role="status">
        <FileQuestion aria-hidden="true" />
        <p>尚未选择文档</p>
      </section>
    );
  }
  // A zero-byte, whitespace-only, or frontmatter-only Markdown document renders
  // an empty body. The server already strips frontmatter from the body it
  // returns, so trimming the rendered HTML is the complete emptiness test — and
  // frontmatter stays visible above the empty state: it is valid document
  // metadata, only the body has nothing to show.
  const documentEmpty = html.trim() === "";
  return (
    <>
      {frontmatter !== null && frontmatter.entries.length > 0 ? (
        <div className="reader-frontmatter">
          <FrontMatterPanel frontmatter={frontmatter} />
        </div>
      ) : null}
      {documentEmpty ? (
        <section className="state-panel" role="status">
          <FileText aria-hidden="true" />
          <p>当前文档无内容</p>
        </section>
      ) : (
        <article
          ref={contentRef}
          className="markdown-body reader-document"
          onClick={handleContentClick}
          onKeyDown={onKeyDown}
          onErrorCapture={onErrorCapture}
        />
      )}
      {lightbox !== null ? (
        <DocumentLightbox
          items={lightbox.items}
          index={lightbox.index}
          open={lightboxOpen}
          onIndexChange={(index) =>
            setLightbox((previous) =>
              previous === null ? previous : { ...previous, index },
            )
          }
          onClose={() => setLightboxOpen(false)}
          onClosed={() => setLightbox(null)}
        />
      ) : null}
    </>
  );
}
