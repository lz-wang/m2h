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
import { renderRichContent } from "./lib/render-rich-content";
import { type DocumentWidth, type Mode, readRoute } from "./model";
import { useDirectoryPreview } from "./use-directory-preview";
import { usePreviewEvents } from "./use-preview-events";
import { useTocSpy } from "./use-toc-spy";

interface AppProps {
  api?: PreviewAPI;
}

const modes: Array<{ value: Mode; label: string; icon: typeof Sun }> = [
  { value: "auto", label: "跟随系统", icon: SunMoon },
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
];

const layoutStorageKey = "m2h.preview.layout";

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
  const preview = useDirectoryPreview(api);
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
  // The TOC lists H2-H4 only: the first H1 is already the document title shown
  // in the toolbar, and H5/H6 are too deep for a narrow rail.
  const tocItems = useMemo<TocItem[]>(
    () =>
      (preview.document?.toc ?? []).filter(
        (item) => item.level >= 2 && item.level <= 4,
      ),
    [preview.document],
  );
  const tocVisible = preview.toc && tocItems.length > 0;
  const readerMainRef = useRef<HTMLDivElement>(null);
  const activeHeadingID = useTocSpy(tocItems, readerMainRef, tocVisible);

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
                  onRetry={() => void preview.retry()}
                  onClick={handleMarkdownClick}
                  onKeyDown={handleMarkdownKeyDown}
                  onErrorCapture={handleAssetError}
                />
              </div>
            </ScrollArea>
            {tocVisible ? (
              <TableOfContentsPanel
                items={tocItems}
                activeID={activeHeadingID}
              />
            ) : null}
          </div>
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
    <Menu.Root>
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
    <Menu.Root>
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
  phase: ReturnType<typeof useDirectoryPreview>["phase"];
  error: string | null;
  html: string | null;
  frontmatter: FrontMatter | null;
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
  onRetry,
  onClick,
  onKeyDown,
  onErrorCapture,
}: PreviewContentProps) {
  const contentRef = useRef<HTMLElement>(null);
  const renderGenerationRef = useRef(0);

  // React owns the <article> container; the Markdown body DOM is owned by
  // the rich-content renderer. Writing innerHTML in a layout effect keeps UI
  // state changes (theme, width, sidebar) from resetting KaTeX/Mermaid
  // enhancements, and runs before paint so there is no empty-body flash.
  //
  // The effect keys on `phase` as well as `html`. Only the "ready" phase
  // renders the <article>; re-entering "ready" (e.g. refreshing the current
  // document, where `html` is the same string) remounts the node, so without
  // `phase` the fresh <article> would render empty. Theme, width and sidebar
  // changes never alter `html` or `phase`, so they cannot disturb the DOM.
  //
  // The generation guard pairs with renderRichContent's freshness check so a
  // slow Mermaid render that outlives its document does not apply KaTeX after
  // the body has been swapped; cleanup invalidates the in-flight render.
  useLayoutEffect(() => {
    if (phase !== "ready") {
      return;
    }
    const root = contentRef.current;
    if (root === null || html === null) {
      return;
    }
    const generation = ++renderGenerationRef.current;
    root.innerHTML = html;
    void renderRichContent(
      root,
      () => renderGenerationRef.current === generation,
    );
    return () => {
      if (renderGenerationRef.current === generation) {
        renderGenerationRef.current++;
      }
    };
  }, [html, phase]);

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
