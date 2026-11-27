import {
  FileQuestion,
  FileText,
  Inbox,
  LoaderCircle,
  RefreshCw,
  TriangleAlert,
} from "lucide-react";
import {
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type SyntheticEvent,
  useEffect,
  useEffectEvent,
  useLayoutEffect,
  useRef,
  useState,
} from "react";

import type { FrontMatter } from "../api";
import {
  collectLightboxState,
  type LightboxState,
} from "../lib/document-lightbox";
import { enhanceDocumentLinks } from "../lib/document-links";
import { observeLazyImages, prepareLazyImages } from "../lib/lazy-images";
import {
  renderRichContent,
  rerenderMermaid,
  revealLazyImage,
} from "../lib/render-rich-content";
import type { ResolvedMode } from "../model";
import type { usePreview } from "../use-preview";
import { DocumentLightbox } from "./document-lightbox";
import { FrontMatterPanel } from "./frontmatter";
import { Button } from "./ui/button";

interface PreviewContentProps {
  phase: ReturnType<typeof usePreview>["phase"];
  error: string | null;
  html: string | null;
  frontmatter: FrontMatter | null;
  resolvedMode: ResolvedMode;
  onRetry(): void;
  onClick?(event: ReactMouseEvent<HTMLElement>): void;
  onKeyDown?(event: ReactKeyboardEvent<HTMLElement>): void;
  onErrorCapture?(event: SyntheticEvent<HTMLElement>): void;
  onVisualError(message: string): void;
  // Notified once the mounted body's rich-content rendering has settled —
  // after Mermaid/KaTeX/table enhancement, so annotation anchoring sees
  // the final DOM shape. Only the "ready" phase fires it.
  onBodyReady?(root: HTMLElement): void;
}

export function PreviewContent({
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
  onBodyReady,
}: PreviewContentProps) {
  const contentRef = useRef<HTMLElement>(null);
  // Body generation: bumped on every body swap and teardown, never by a
  // theme switch. It is the freshness token handed to the renderers below —
  // a slow render that outlives its document must not apply after the body
  // has moved on — while a theme switch queues behind the initial render
  // instead of invalidating it (see the theme effect below).
  const bodyGenerationRef = useRef(0);
  const initialRenderRef = useRef<Promise<void> | null>(null);
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
  // the rich-content renderer. Mounting the parsed body in a layout effect
  // runs before paint so there is no empty-body flash.
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
  // rich visuals (Mermaid diagrams), whose colors are
  // baked into their SVG at render time. The body generation guard pairs
  // with the renderers' freshness checks so a slow render that outlives its
  // document is not applied after the body has moved on; cleanup invalidates
  // the in-flight render.
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
    // The body is parsed inside a <template> and mounted as a fragment, so
    // every image's real source is parked before the document can see it —
    // mounting server HTML directly would start every image request before
    // any lazy scheduling could run. See lib/lazy-images.ts.
    root.replaceChildren(prepareLazyImages(html));
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
    // The images may load now: the observer hands each parked source back as
    // its image approaches the viewport, and a loaded image's presentation
    // (metadata tooltip, Lightbox) follows through the reveal hook. Only the
    // body's own lifetime owns the observation — the next body swap
    // disconnects it with the rest of the cleanup.
    const lazyImages = observeLazyImages(root, {
      onImageSettled: (image, loaded) => {
        if (loaded) {
          revealLazyImage(image);
        }
      },
    });
    return () => {
      bodyGenerationRef.current++;
      initialRenderRef.current = null;
      lazyImages.disconnect();
    };
  }, [html, phase, onVisualError]);

  // Annotation anchoring waits for the rich-content pass: Mermaid and
  // KaTeX replace text nodes, so indexing any earlier would anchor against
  // DOM that is about to disappear. The body generation guard keeps a slow
  // runtime from notifying about a body the reader has already left, and a
  // theme switch never re-fires this (the body effect does not re-run).
  const notifyBodyReady = useEffectEvent((root: HTMLElement) =>
    onBodyReady?.(root),
  );
  const needsBodyIndex = onBodyReady !== undefined;
  useEffect(() => {
    // `html` re-arms this effect on every body swap; the generation guard
    // below discards notifications from a body the reader has left.
    void html;
    void onVisualError;
    if (phase !== "ready" || !needsBodyIndex) {
      return;
    }
    let cancelled = false;
    const generation = bodyGenerationRef.current;
    void initialRenderRef.current?.then(() => {
      const root = contentRef.current;
      if (
        !cancelled &&
        root !== null &&
        bodyGenerationRef.current === generation &&
        root.isConnected
      ) {
        notifyBodyReady(root);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [html, phase, needsBodyIndex, onVisualError]);

  // A theme switch repaints only Mermaid diagrams, leaving the article DOM,
  // KaTeX, copy buttons, and any in-body focus untouched. On the initial render
  // the body effect has already painted the current theme, so renderedModeRef
  // matches resolvedMode and this effect is a no-op; it only does work on a
  // subsequent light/dark toggle.
  //
  // The repaint is queued, never run inline. Each queued repaint waits for
  // the initial enhancement of the current body — a toggle while a runtime
  // is still downloading must wait for the first Mermaid render to finish —
  // and for any earlier repaint, so one diagram never has two paints in flight.
  // A queued repaint is skipped when a newer toggle has claimed a different theme (rapid light → dark → light ends with one
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
      await rerenderMermaid(
        root,
        mode,
        () => bodyGenerationRef.current === generation,
        onVisualError,
      );
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
      // Image frames and rich-visual frames (Mermaid)
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
    onClick?.(event);
  };

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
        <span>添加 .md 或 .markdown 文件后，点击「刷新文档首页」。</span>
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
