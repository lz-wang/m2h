import { Clock3, FileText, Folder, LoaderCircle } from "lucide-react";
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
  revision: number;
  forceRefresh: boolean;
  onRecentSettled(revision: number): void;
  fileCount: number;
  roots: RootSummary[];
  options: ViewOptions;
  onSelect(path: string): void;
}

export function HomePage({
  getRecent,
  revision,
  forceRefresh,
  onRecentSettled,
  fileCount,
  roots,
  options,
  onSelect,
}: HomePageProps) {
  const [result, setResult] = useState<{
    revision: number;
    recent: RecentResponse | "error";
  } | null>(null);
  const recent = result?.revision === revision ? result.recent : null;
  const loading = recent === null;

  useEffect(() => {
    if (!loading) return;
    const controller = new AbortController();
    const settle = (recent: RecentResponse | "error") => {
      if (controller.signal.aborted) return;
      setResult({ revision, recent });
      onRecentSettled(revision);
    };
    void getRecent(controller.signal, forceRefresh).then(settle, () =>
      settle("error"),
    );
    return () => controller.abort();
  }, [getRecent, loading, revision, forceRefresh, onRecentSettled]);

  const lists = recent === "error" ? null : recent;
  const latestDate = lists?.updated[0]?.date;

  return (
    <article className="document-home" aria-labelledby="document-home-title">
      <h1 id="document-home-title" className="sr-only">
        文档首页
      </h1>
      <div className="document-home-layout">
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
              文档源
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
            <Button variant="outline" onClick={() => setResult(null)}>
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
                <h2 id={id}>{label}</h2>
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
