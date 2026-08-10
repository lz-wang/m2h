import { Menu } from "@base-ui/react/menu";
import {
  Check,
  FileQuestion,
  ImageOff,
  Inbox,
  LoaderCircle,
  MonitorCog,
  Moon,
  RefreshCw,
  Sun,
  SunMoon,
  TriangleAlert,
} from "lucide-react";
import type {
  KeyboardEvent as ReactKeyboardEvent,
  MouseEvent as ReactMouseEvent,
  SyntheticEvent,
} from "react";

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

export function App({ api }: AppProps) {
  const preview = useDirectoryPreview(api);
  const loading =
    preview.phase === "loading-files" || preview.phase === "loading-document";

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
      <SidebarProvider className="app-shell">
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
        </Sidebar>

        <SidebarInset className="reader-inset">
          <header className="reader-toolbar">
            <SidebarTrigger aria-label="切换文件导航" />
            <Separator orientation="vertical" className="toolbar-separator" />
            <DocumentPath path={preview.selectedPath} />
            <div className="toolbar-actions">
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
            <div className="reader-canvas">
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

function DocumentPath({ path }: { path: string | null }) {
  const label = path ?? "未选择文档";
  return (
    <nav
      className="document-path"
      aria-label="当前文档路径"
      title={path ?? undefined}
    >
      {label}
    </nav>
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

  const handleChange = (value: string) => {
    if (value === "light" || value === "dark" || value === "auto") {
      onChange(value);
    }
  };

  return (
    <Menu.Root>
      <Menu.Trigger
        render={
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`显示主题：${currentLabel}`}
          />
        }
      >
        <SunMoon aria-hidden="true" />
      </Menu.Trigger>
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
