import { ImageOff } from "lucide-react";
import {
  type CSSProperties,
  useCallback,
  useEffect,
  useRef,
  useState,
} from "react";
import { DocumentWidthMenu, ThemeMenu } from "../../App";
import { APIError, getSharedDocument } from "../../api";
import { useTheme } from "../../hooks/use-theme";
import { activateTabForElement } from "../../lib/tabbed";
import { decodeHeadingHash, encodeHeadingHash, readRoute } from "../../model";
import { useHeadingNavigation } from "../../use-heading-navigation";
import { useHeadingSpy } from "../../use-heading-spy";
import { PreviewContent } from "../preview-content";
import { ReaderFooter } from "../reader-footer";
import {
  TableOfContentsPanel,
  TableOfContentsSheet,
  TOCToggle,
} from "../table-of-contents";
import { TooltipProvider } from "../ui/tooltip";

type PublicDocument = Awaited<ReturnType<typeof getSharedDocument>>;
type PublicState =
  | { phase: "loading-document" | "not-found" | "error"; document: null }
  | { phase: "ready"; document: PublicDocument };

export function PublicReader({ id }: { id: string }) {
  const [preferences] = useState(() =>
    readRoute(location.pathname, location.search),
  );
  const [mode, setMode] = useState(preferences.mode);
  const [width, setWidth] = useState(preferences.width);
  const [toc, setToc] = useState(preferences.toc);
  const resolvedMode = useTheme(mode);
  const [state, setState] = useState<PublicState>({
    phase: "loading-document",
    document: null,
  });
  const [revision, setRevision] = useState(0);
  const [visualError, setVisualError] = useState<string | null>(null);
  const container = useRef<HTMLDivElement>(null);
  const headings = state.document?.toc ?? [];
  const active = useHeadingSpy(headings, container, state.phase === "ready");
  const navigate = useHeadingNavigation(container, (heading) => {
    history.replaceState(
      history.state,
      "",
      location.pathname + location.search + encodeHeadingHash(heading ?? ""),
    );
  });

  useEffect(() => {
    void revision;
    const controller = new AbortController();
    setState({ phase: "loading-document", document: null });
    setVisualError(null);
    void getSharedDocument(id, controller.signal).then(
      (document) => {
        if (!controller.signal.aborted) setState({ phase: "ready", document });
      },
      (error: unknown) => {
        if (!controller.signal.aborted) {
          setState({
            phase:
              error instanceof APIError && error.status === 404
                ? "not-found"
                : "error",
            document: null,
          });
        }
      },
    );
    return () => controller.abort();
  }, [id, revision]);

  useEffect(() => {
    if (state.document !== null)
      document.title = `${state.document.title} · m2h`;
  }, [state.document]);

  const restoreAnchor = useCallback((body: HTMLElement) => {
    const anchor = decodeHeadingHash(location.hash);
    const target = anchor ? document.getElementById(anchor) : null;
    if (target !== null && body.contains(target)) {
      activateTabForElement(target);
      target.scrollIntoView({ block: "start" });
    }
  }, []);

  function navigateToHeading(heading: string) {
    navigate(heading, { behavior: "smooth" });
  }

  return (
    <TooltipProvider>
      <div
        className="reader-inset"
        style={{ "--reader-toc-width": "240px" } as CSSProperties}
      >
        <header className="reader-toolbar">
          <span className="min-w-0 flex-1 truncate px-2 text-sm font-medium">
            {state.document?.title ?? "公开分享"}
          </span>
          <div className="toolbar-actions">
            {headings.length > 0 ? (
              <>
                <TOCToggle enabled={toc} available onChange={setToc} />
                <TableOfContentsSheet
                  items={headings}
                  activeID={active}
                  onNavigate={navigateToHeading}
                />
              </>
            ) : null}
            <DocumentWidthMenu width={width} onChange={setWidth} />
            <ThemeMenu mode={mode} onChange={setMode} />
          </div>
        </header>
        <main className="reader-main" ref={container}>
          <div className={`reader-canvas reader-canvas-${width}`}>
            {visualError !== null ? (
              <div className="asset-warning" role="status">
                <ImageOff aria-hidden="true" />
                <span>{visualError}</span>
              </div>
            ) : null}
            {state.phase === "not-found" ? (
              <section className="state-panel" role="status">
                <p>此分享链接不可用。</p>
              </section>
            ) : (
              <PreviewContent
                phase={state.phase}
                html={state.document?.html ?? null}
                frontmatter={null}
                error="无法加载分享文档，请稍后重试。"
                resolvedMode={resolvedMode}
                onRetry={() => setRevision((value) => value + 1)}
                onVisualError={setVisualError}
                onBodyReady={restoreAnchor}
                onClick={(event) => {
                  const link =
                    event.target instanceof Element
                      ? event.target.closest("a[href^='#']")
                      : null;
                  const anchor = decodeHeadingHash(
                    link?.getAttribute("href") ?? "",
                  );
                  const target = anchor
                    ? document.getElementById(anchor)
                    : null;
                  if (target !== null && event.currentTarget.contains(target))
                    activateTabForElement(target);
                }}
              />
            )}
            {state.document !== null ? <ReaderFooter version="" /> : null}
          </div>
          {headings.length > 0 ? (
            <div className="reader-rail-slot" data-open={toc}>
              <div
                className="reader-rail"
                data-toc-open={toc}
                data-annotations-open="false"
              >
                <TableOfContentsPanel
                  visible={toc}
                  items={headings}
                  activeID={active}
                  onNavigate={navigateToHeading}
                />
              </div>
            </div>
          ) : null}
        </main>
      </div>
    </TooltipProvider>
  );
}
