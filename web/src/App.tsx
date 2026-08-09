import {
  FileQuestion,
  ImageOff,
  Inbox,
  LoaderCircle,
  MonitorCog,
  Moon,
  RefreshCw,
  Sun,
  TriangleAlert,
} from "lucide-react";
import type {
  CSSProperties,
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
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
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
  { value: "light", label: "浅色", icon: Sun },
  { value: "dark", label: "深色", icon: Moon },
  { value: "auto", label: "跟随系统", icon: MonitorCog },
];

export function App({ api }: AppProps) {
  const preview = useDirectoryPreview(api);
  const loading =
    preview.phase === "loading-files" || preview.phase === "loading-document";
  const fileCountLabel = `${preview.files.length} 篇文档`;

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
        style={
          {
            "--sidebar-width": "20rem",
            "--sidebar-width-icon": "3.25rem",
          } as CSSProperties
        }
      >
        <Sidebar collapsible="offcanvas" className="sidebar-ledger">
          <SidebarHeader className="ledger-header">
            <div className="brand-lockup">
              <div className="brand-mark" aria-hidden="true">
                <span>M</span>
                <i>2</i>
                <span>H</span>
              </div>
              <div>
                <p className="brand-kicker">LOCAL MARKDOWN INDEX</p>
                <p className="brand-name">m2h archive</p>
              </div>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      variant="ghost"
                      size="icon"
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
                <TooltipContent side="right">重新扫描目录</TooltipContent>
              </Tooltip>
            </div>
            <Separator />
            <div className="ledger-meta">
              <span>INDEX / 01</span>
              <span aria-live="polite">{fileCountLabel}</span>
            </div>
          </SidebarHeader>
          <SidebarContent>
            <ScrollArea className="tree-scroll">
              <SidebarGroup>
                <SidebarGroupContent>
                  {preview.files.length > 0 ? (
                    <DocumentTree
                      files={preview.files}
                      selectedPath={preview.selectedPath}
                      onSelect={(path) => void preview.select(path)}
                    />
                  ) : (
                    <p className="tree-placeholder">
                      {loading ? "正在编制索引…" : "目录中没有 Markdown"}
                    </p>
                  )}
                </SidebarGroupContent>
              </SidebarGroup>
            </ScrollArea>
          </SidebarContent>
          <SidebarFooter className="ledger-footer">
            <p>depth & glob 由服务端统一执行</p>
            <p>⌘ / Ctrl + B 切换索引</p>
          </SidebarFooter>
        </Sidebar>

        <SidebarInset className="reader-inset">
          <header className="reader-header">
            <div className="reader-heading-row">
              <SidebarTrigger
                aria-label="切换文件导航"
                className="sidebar-trigger"
              />
              <div className="reader-heading-copy">
                <p className="document-path">
                  {preview.selectedPath ?? "NO DOCUMENT SELECTED"}
                </p>
                <h1 title={preview.document?.title}>
                  {preview.document?.title ?? "Markdown archive"}
                </h1>
              </div>
              <fieldset className="mode-switcher">
                <legend className="sr-only">显示主题</legend>
                {modes.map(({ value, label, icon: Icon }) => (
                  <Button
                    key={value}
                    variant="ghost"
                    size="icon-sm"
                    aria-label={label}
                    aria-pressed={preview.mode === value}
                    onClick={() => preview.setMode(value)}
                  >
                    <Icon aria-hidden="true" />
                  </Button>
                ))}
              </fieldset>
            </div>
            <div className="header-rule" aria-hidden="true">
              <span />
              <i />
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
        <p>{phase === "loading-files" ? "正在建立文档索引" : "正在装订文档"}</p>
        <span>内容始终从当前磁盘版本读取</span>
      </section>
    );
  }
  if (phase === "empty") {
    return (
      <section className="state-panel" aria-live="polite">
        <Inbox aria-hidden="true" />
        <p>这个目录还是空的</p>
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
