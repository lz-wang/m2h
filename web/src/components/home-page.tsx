import { Clock3, FileText, Folder, Link2, LoaderCircle } from "lucide-react";
import { type MouseEvent, useEffect, useState } from "react";
import type {
  PreviewAPI,
  RecentDocument,
  RecentResponse,
  RootSummary,
} from "../api";
import { routeURL, type ViewOptions } from "../model";
import { Button } from "./ui/button";

import "./home-page.css";

const sections = [
  { key: "created", id: "latest-created", label: "最新创建" },
  { key: "updated", id: "recently-updated", label: "最近更新" },
] as const;

interface HomePageProps {
  getRecent: PreviewAPI["getRecent"];
  fileCount: number;
  roots: RootSummary[];
  options: ViewOptions;
  onSelect(path: string): void;
}

export function HomePage({
  getRecent,
  fileCount,
  roots,
  options,
  onSelect,
}: HomePageProps) {
  const [recent, setRecent] = useState<RecentResponse | "loading" | "error">(
    "loading",
  );
  const loading = recent === "loading";

  useEffect(() => {
    if (!loading) return;
    const controller = new AbortController();
    void getRecent(controller.signal).then(
      (response) => {
        if (!controller.signal.aborted) setRecent(response);
      },
      () => {
        if (!controller.signal.aborted) setRecent("error");
      },
    );
    return () => controller.abort();
  }, [getRecent, loading]);

  const lists = typeof recent === "object" ? recent : null;
  const latestDate = lists?.updated[0]?.date;

  return (
    <article className="document-home" aria-labelledby="document-home-title">
      <div className="document-home-layout">
        <div className="document-home-content">
          <header className="document-home-header">
            <Link2 aria-hidden="true" />
            <div>
              <h1 id="document-home-title" tabIndex={-1}>
                文档首页
              </h1>
              <p>浏览最近新增和更新的 Markdown 文档。</p>
            </div>
          </header>

          <dl className="document-home-overview">
            <div>
              <dt>
                <FileText aria-hidden="true" />
                <span>
                  <span className="document-home-format">Markdown </span>文档
                </span>
              </dt>
              <dd>{fileCount.toLocaleString()}</dd>
            </div>
            <div>
              <dt>
                <Clock3 aria-hidden="true" />
                最近更新
              </dt>
              <dd>
                {latestDate ? (
                  <time dateTime={latestDate}>{latestDate}</time>
                ) : (
                  "—"
                )}
              </dd>
            </div>
            <div>
              <dt>
                <Folder aria-hidden="true" />
                目录根
              </dt>
              <dd>{roots.length.toLocaleString()}</dd>
            </div>
          </dl>

          {loading ? (
            <p className="document-home-status" role="status">
              <LoaderCircle className="is-spinning" aria-hidden="true" />
              正在加载最近文档…
            </p>
          ) : null}
          {recent === "error" ? (
            <div className="document-home-status" role="alert">
              无法读取最近文档。
              <Button variant="outline" onClick={() => setRecent("loading")}>
                重试
              </Button>
            </div>
          ) : null}

          {lists !== null
            ? sections.map(({ key, id, label }) => (
                <section
                  className="document-home-section"
                  key={key}
                  aria-labelledby={id}
                >
                  <h2 id={id} tabIndex={-1}>
                    {label}
                  </h2>
                  {lists[key].length > 0 ? (
                    <ul className="document-home-list">
                      {lists[key].map((entry) => (
                        <li key={entry.path}>
                          <RecentLink
                            entry={entry}
                            directory={documentDirectory(entry.path, roots)}
                            estimated={
                              key === "created" && entry.dateSource === "mtime"
                            }
                            options={options}
                            onSelect={onSelect}
                          />
                        </li>
                      ))}
                    </ul>
                  ) : (
                    <p className="document-home-status">暂无可展示的文档。</p>
                  )}
                </section>
              ))
            : null}
        </div>

        {lists !== null && options.toc ? (
          <nav className="document-home-outline" aria-label="首页导航">
            <span>本页导航</span>
            <button
              type="button"
              onClick={() => jumpToHomeSection("document-home-title")}
            >
              首页概览
            </button>
            {sections.map(({ id, label }) => (
              <button
                type="button"
                key={id}
                onClick={() => jumpToHomeSection(id)}
              >
                {label}
              </button>
            ))}
          </nav>
        ) : null}
      </div>
    </article>
  );
}

function RecentLink({
  entry,
  directory,
  estimated,
  options,
  onSelect,
}: {
  entry: RecentDocument;
  directory: string;
  estimated: boolean;
  options: ViewOptions;
  onSelect(path: string): void;
}) {
  return (
    <a
      className="recent-document-link"
      href={routeURL(entry.path, options)}
      onClick={(event) => {
        if (!ordinaryClick(event)) return;
        event.preventDefault();
        onSelect(entry.path);
      }}
    >
      <span className="recent-document-title">
        <FileText aria-hidden="true" />
        {entry.title}
      </span>
      <span className="recent-document-directory">{directory}</span>
      <span className="recent-document-date">
        <time dateTime={entry.date}>{entry.date}</time>
        {estimated ? (
          <span className="recent-date-source" title="以文件修改时间估算">
            估算
          </span>
        ) : null}
      </span>
    </a>
  );
}

function documentDirectory(path: string, roots: RootSummary[]): string {
  const parts = path.split("/").slice(0, -1);
  if (roots.length > 1) {
    const root = roots.find((root) => root.id === parts[0]);
    if (root !== undefined) parts[0] = root.name;
  }
  return parts.length > 0 ? parts.join("/") : "根目录";
}

function ordinaryClick(event: MouseEvent<HTMLAnchorElement>): boolean {
  return (
    event.button === 0 &&
    !event.metaKey &&
    !event.ctrlKey &&
    !event.shiftKey &&
    !event.altKey
  );
}

function jumpToHomeSection(id: string): void {
  const section = document.getElementById(id);
  section?.scrollIntoView();
  section?.focus({ preventScroll: true });
}
