import { Menu } from "@base-ui/react/menu";
import {
  Check,
  Columns3,
  FileQuestion,
  ImageOff,
  Inbox,
  LoaderCircle,
  Maximize2,
  Moon,
  RefreshCw,
  Scaling,
  Search,
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
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react";

import type { FrontMatter, PreviewAPI, TocItem } from "./api";
import { DocumentTree } from "./components/document-tree";
import { FrontMatterPanel, FrontMatterSummary } from "./components/frontmatter";
import {
  TableOfContentsPanel,
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
import { renderRichContent, rerenderMermaid } from "./lib/render-rich-content";
import {
  type DocumentWidth,
  type Mode,
  type ResolvedMode,
  readRoute,
} from "./model";
import { useHeadingSpy } from "./use-heading-spy";
import { usePreview } from "./use-preview";
import { usePreviewEvents } from "./use-preview-events";

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
  usePreviewEvents(preview.reloadCurrent);
  // A single-file scope has nothing to switch between, so the file sidebar and
  // its toolbar trigger stay hidden; every other control remains shared.
  const navigationAvailable = preview.kind === "directory";
  const [initialLayout] = useState(readStoredLayout);
  const [sidebarWidth, setSidebarWidth] = useState(initialLayout.sidebarWidth);
  const [sidebarOpen, setSidebarOpen] = useState(initialLayout.sidebarOpen);
  const [sidebarResizing, setSidebarResizing] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const filteredFiles = useMemo(() => {
    const query = searchQuery.trim().toLocaleLowerCase();
    if (query === "") {
      return preview.files;
    }
    return preview.files.filter(
      (file) =>
        file.name.toLocaleLowerCase().includes(query) ||
        file.title.toLocaleLowerCase().includes(query),
    );
  }, [preview.files, searchQuery]);
  const loading =
    preview.phase === "loading-files" || preview.phase === "loading-document";
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
    if (
      target.origin !== window.location.origin ||
      !target.pathname.startsWith("/doc/")
    ) {
      return false;
    }
    const route = readRoute(target.pathname, target.search, target.hash);
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
      preview.reportAssetError(event.target.getAttribute("src") ?? "");
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
                <Search aria-hidden="true" />
                <Input
                  type="search"
                  value={searchQuery}
                  aria-label="搜索文档"
                  placeholder="搜索标题或文件名"
                  onChange={(event) => setSearchQuery(event.target.value)}
                />
              </div>
            </SidebarHeader>
            <SidebarContent>
              <ScrollArea className="tree-scroll">
                <SidebarGroup>
                  <SidebarGroupLabel className="justify-between">
                    <span>Files</span>
                    <span className="text-xs tabular-nums text-sidebar-foreground/60">
                      <span aria-hidden="true">{filteredFiles.length}</span>
                      <span className="sr-only">
                        {filteredFiles.length} 个 Markdown 文件
                      </span>
                    </span>
                  </SidebarGroupLabel>
                  <SidebarGroupContent>
                    {filteredFiles.length > 0 ? (
                      <DocumentTree
                        files={filteredFiles}
                        searching={searchQuery.trim() !== ""}
                        selectedPath={preview.selectedPath}
                        onSelect={(path) => void preview.select(path)}
                      />
                    ) : (
                      <p className="tree-placeholder">
                        {loading
                          ? "正在加载文件…"
                          : searchQuery.trim() !== ""
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
            </div>
          </header>

          <div className="reader-main" ref={readerMainRef}>
            <ScrollArea className="reader-scroll">
              <div className={`reader-canvas reader-canvas-${preview.width}`}>
                {preview.assetError !== null ? (
                  <div className="asset-warning" role="status">
                    <ImageOff aria-hidden="true" />
                    <span>{preview.assetError}</span>
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
                />
              </div>
            </ScrollArea>
            {tocVisible ? (
              <TableOfContentsPanel items={tocItems} activeID={activeTocID} />
            ) : null}
          </div>
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
    <SidebarFooter className="flex-row items-center justify-start gap-1 px-3 py-2">
      <a
        href={repositoryURL}
        target="_blank"
        rel="noreferrer"
        aria-label="在新页面打开 m2h GitHub 仓库"
        title="GitHub"
        className="flex size-8 items-center justify-center rounded-md text-sidebar-foreground/60 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
      >
        <GitHubIcon />
      </a>
      {version !== "" ? (
        <a
          href={versionURL}
          target="_blank"
          rel="noreferrer"
          aria-label={`在新页面打开 m2h ${versionLabel} 发布信息`}
          title="查看发布信息"
          className="rounded-md px-2 py-1 font-mono text-[11px] tabular-nums text-sidebar-foreground/55 transition-colors hover:bg-sidebar-accent hover:text-sidebar-accent-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-sidebar-ring"
        >
          {versionLabel}
        </a>
      ) : null}
    </SidebarFooter>
  );
}

function GitHubIcon() {
  return (
    <svg
      aria-hidden="true"
      className="size-4"
      viewBox="0 0 24 24"
      fill="currentColor"
    >
      <path d="M12 .3a12 12 0 0 0-3.8 23.4c.6.1.8-.3.8-.6v-2.3c-3.3.7-4-1.4-4-1.4-.5-1.4-1.3-1.8-1.3-1.8-1.1-.7.1-.7.1-.7 1.2.1 1.8 1.2 1.8 1.2 1.1 1.8 2.8 1.3 3.5 1 .1-.8.4-1.3.8-1.6-2.7-.3-5.5-1.3-5.5-5.9 0-1.3.5-2.4 1.2-3.2-.1-.3-.5-1.5.1-3.2 0 0 1-.3 3.3 1.2a11.4 11.4 0 0 1 6 0C17.5 5.9 18.5 6.2 18.5 6.2c.7 1.7.3 2.9.1 3.2.8.8 1.2 1.9 1.2 3.2 0 4.6-2.8 5.6-5.5 5.9.4.4.8 1.1.8 2.2v3.3c0 .3.2.7.8.6A12 12 0 0 0 12 .3Z" />
    </svg>
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
}: PreviewContentProps) {
  const contentRef = useRef<HTMLElement>(null);
  const renderGenerationRef = useRef(0);
  // The resolved theme is read through a ref so the body render effect can
  // depend on [html, phase] only and a theme switch never rebuilds the
  // article. renderedModeRef records which theme the current body — and its
  // Mermaid SVGs — were last painted in, so the theme effect can skip when
  // nothing changed and otherwise regenerate only the diagrams.
  const resolvedModeRef = useRef<ResolvedMode>(resolvedMode);
  const renderedModeRef = useRef<ResolvedMode | null>(null);
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
  // The separate theme effect below regenerates only the Mermaid diagrams,
  // whose colors are baked into the SVG at render time. The generation guard
  // pairs with the renderers' freshness checks so a slow Mermaid render that
  // outlives its document (or a later theme toggle) is not applied after the
  // body has moved on; cleanup invalidates the in-flight render.
  useLayoutEffect(() => {
    if (phase !== "ready") {
      return;
    }
    const root = contentRef.current;
    if (root === null || html === null) {
      return;
    }
    const generation = ++renderGenerationRef.current;
    const mode = resolvedModeRef.current;
    root.innerHTML = html;
    renderedModeRef.current = mode;
    void renderRichContent(
      root,
      mode,
      () => renderGenerationRef.current === generation,
    );
    return () => {
      if (renderGenerationRef.current === generation) {
        renderGenerationRef.current++;
      }
    };
  }, [html, phase]);

  // A theme switch repaints only the Mermaid diagrams, leaving the article
  // DOM, KaTeX, copy buttons, and any in-body focus untouched. On the initial
  // render the body effect above has already painted the current theme, so
  // renderedModeRef matches resolvedMode and this effect is a no-op; it only
  // does work on a subsequent light/dark toggle.
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
    const generation = ++renderGenerationRef.current;
    void rerenderMermaid(
      root,
      resolvedMode,
      () => renderGenerationRef.current === generation,
    );
    return () => {
      if (renderGenerationRef.current === generation) {
        renderGenerationRef.current++;
      }
    };
  }, [phase, resolvedMode]);

  if (phase === "loading-files" || phase === "loading-document") {
    return (
      <section className="state-panel" aria-live="polite" aria-busy="true">
        <LoaderCircle className="is-spinning" aria-hidden="true" />
        <p>{phase === "loading-files" ? "正在加载文件" : "正在加载文档"}</p>
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
  return (
    <>
      {frontmatter !== null && frontmatter.entries.length > 0 ? (
        <div className="reader-frontmatter">
          <FrontMatterPanel frontmatter={frontmatter} />
        </div>
      ) : null}
      <article
        ref={contentRef}
        className="markdown-body reader-document"
        onClick={onClick}
        onKeyDown={onKeyDown}
        onErrorCapture={onErrorCapture}
      />
    </>
  );
}
