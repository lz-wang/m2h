import { Menu } from "@base-ui/react/menu";
import {
  Check,
  Columns3,
  FileQuestion,
  ImageOff,
  Inbox,
  LoaderCircle,
  Maximize2,
  MonitorCog,
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
import { useEffect, useState } from "react";

import type { PreviewAPI } from "./api";
import { DocumentTree } from "./components/document-tree";
import { Button } from "./components/ui/button";
import { ScrollArea } from "./components/ui/scroll-area";
import { Separator } from "./components/ui/separator";
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
import { type Mode, readRoute } from "./model";
import { useDirectoryPreview } from "./use-directory-preview";

interface AppProps {
  api?: PreviewAPI;
}

const modes: Array<{ value: Mode; label: string; icon: typeof Sun }> = [
  { value: "auto", label: "跟随系统", icon: MonitorCog },
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
];

type DocumentWidth = "standard" | "wide" | "full";

const layoutStorageKey = "m2h.preview.layout";

interface StoredLayout {
  sidebarOpen: boolean;
  sidebarWidth: number;
  documentWidth: DocumentWidth;
}

const defaultLayout: StoredLayout = {
  sidebarOpen: true,
  sidebarWidth: 256,
  documentWidth: "standard",
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
  const [initialLayout] = useState(readStoredLayout);
  const [documentWidth, setDocumentWidth] = useState<DocumentWidth>(
    initialLayout.documentWidth,
  );
  const [sidebarWidth, setSidebarWidth] = useState(initialLayout.sidebarWidth);
  const [sidebarOpen, setSidebarOpen] = useState(initialLayout.sidebarOpen);
  const loading =
    preview.phase === "loading-files" || preview.phase === "loading-document";

  useEffect(() => {
    try {
      window.localStorage.setItem(
        layoutStorageKey,
        JSON.stringify({ sidebarOpen, sidebarWidth, documentWidth }),
      );
    } catch {
      // Storage can be unavailable in private browsing; the layout still works.
    }
  }, [documentWidth, sidebarOpen, sidebarWidth]);

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
        <Sidebar collapsible="offcanvas">
          <SidebarHeader className="border-b border-sidebar-border">
            <div className="flex h-8 items-center px-2">
              <span className="text-sm font-semibold">m2h</span>
            </div>
          </SidebarHeader>
          <SidebarContent>
            <ScrollArea className="tree-scroll">
              <SidebarGroup>
                <SidebarGroupLabel className="justify-between">
                  <span>Files</span>
                  <span className="text-xs tabular-nums text-sidebar-foreground/60">
                    <span aria-hidden="true">{preview.files.length}</span>
                    <span className="sr-only">
                      {preview.files.length} 个 Markdown 文件
                    </span>
                  </span>
                </SidebarGroupLabel>
                <SidebarGroupContent>
                  {preview.files.length > 0 ? (
                    <DocumentTree
                      files={preview.files}
                      selectedPath={preview.selectedPath}
                      onSelect={(path) => void preview.select(path)}
                    />
                  ) : (
                    <p className="tree-placeholder">
                      {loading ? "正在加载文件…" : "目录中没有 Markdown 文件"}
                    </p>
                  )}
                </SidebarGroupContent>
              </SidebarGroup>
            </ScrollArea>
          </SidebarContent>
          <SidebarResizeHandle
            width={sidebarWidth}
            onResize={setSidebarWidth}
          />
        </Sidebar>

        <SidebarInset className="reader-inset">
          <header className="reader-toolbar">
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
            <Separator orientation="vertical" className="toolbar-separator" />
            <DocumentTitle
              title={preview.document?.title ?? null}
              path={preview.selectedPath}
            />
            <div className="toolbar-actions">
              <DocumentWidthMenu
                width={documentWidth}
                onChange={setDocumentWidth}
              />
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon-sm"
                      aria-label="刷新文件列表"
                      disabled={preview.phase === "loading-files"}
                      onClick={() => void preview.refresh()}
                    >
                      <RefreshCw
                        className={
                          preview.phase === "loading-files" ? "is-spinning" : ""
                        }
                      />
                    </Button>
                  }
                />
                <TooltipContent side="bottom">重新扫描目录</TooltipContent>
              </Tooltip>
              <ThemeMenu mode={preview.mode} onChange={preview.setMode} />
            </div>
          </header>

          <ScrollArea className="reader-scroll">
            <div className={`reader-canvas reader-canvas-${documentWidth}`}>
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
                onRetry={() => void preview.retry()}
                onClick={handleMarkdownClick}
                onKeyDown={handleMarkdownKeyDown}
                onErrorCapture={handleAssetError}
              />
            </div>
          </ScrollArea>
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
    const documentWidth = candidate.documentWidth;
    return {
      sidebarOpen:
        typeof candidate.sidebarOpen === "boolean"
          ? candidate.sidebarOpen
          : defaultLayout.sidebarOpen,
      sidebarWidth:
        typeof sidebarWidth === "number" && Number.isFinite(sidebarWidth)
          ? Math.min(480, Math.max(208, sidebarWidth))
          : defaultLayout.sidebarWidth,
      documentWidth:
        documentWidth === "standard" ||
        documentWidth === "wide" ||
        documentWidth === "full"
          ? documentWidth
          : defaultLayout.documentWidth,
    };
  } catch {
    return defaultLayout;
  }
}

function DocumentTitle({
  title,
  path,
}: {
  title: string | null;
  path: string | null;
}) {
  const label = title ?? "未选择文档";
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <section className="document-title" aria-label="当前文档标题" />
        }
      >
        {label}
      </TooltipTrigger>
      <TooltipContent side="bottom">
        {path ?? "当前没有选择文档"}
      </TooltipContent>
    </Tooltip>
  );
}

function SidebarResizeHandle({
  width,
  onResize,
}: {
  width: number;
  onResize(width: number): void;
}) {
  const handlePointerDown = (event: ReactPointerEvent<HTMLButtonElement>) => {
    event.preventDefault();
    const startX = event.clientX;
    const startWidth = width;
    const handlePointerMove = (moveEvent: PointerEvent) => {
      onResize(
        Math.min(480, Math.max(208, startWidth + moveEvent.clientX - startX)),
      );
    };
    const handlePointerUp = () => {
      window.removeEventListener("pointermove", handlePointerMove);
      window.removeEventListener("pointerup", handlePointerUp);
    };
    window.addEventListener("pointermove", handlePointerMove);
    window.addEventListener("pointerup", handlePointerUp);
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
  onRetry(): void;
  onClick(event: ReactMouseEvent<HTMLElement>): void;
  onKeyDown(event: ReactKeyboardEvent<HTMLElement>): void;
  onErrorCapture(event: SyntheticEvent<HTMLElement>): void;
}

function PreviewContent({
  phase,
  error,
  html,
  onRetry,
  onClick,
  onKeyDown,
  onErrorCapture,
}: PreviewContentProps) {
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
        <span>添加 .md 或 .markdown 文件后点击刷新。</span>
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
    <article
      className="markdown-body reader-document"
      onClick={onClick}
      onKeyDown={onKeyDown}
      onErrorCapture={onErrorCapture}
      dangerouslySetInnerHTML={{ __html: html }}
    />
  );
}
