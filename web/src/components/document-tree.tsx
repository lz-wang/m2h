import { ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import type { CSSProperties, ReactNode } from "react";
import { useLayoutEffect, useMemo, useRef, useState } from "react";
import type { FileSummary } from "@/api";
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
} from "@/components/ui/sidebar";
import {
  ancestorDirectories,
  buildTree,
  type DirectoryNode,
  type TreeNode,
} from "@/model";

// Join a server-reported absolute root path with a root-relative slash path
// using the SERVER machine's separator: the browser may run on a different
// machine, so the platform's native format must come from the API, never from
// the client's own OS sniffing.
function localPath(root: string, relative: string, separator: string): string {
  if (relative === "") {
    return root;
  }
  const suffix = relative.split("/").join(separator);
  if (root.endsWith(separator)) {
    return `${root}${suffix}`;
  }
  return `${root}${separator}${suffix}`;
}

interface DocumentTreeProps {
  // Files of ONE root, root-relative: a multi-root workspace renders one tree
  // per root and keeps display paths free of the root-id prefix.
  files: FileSummary[];
  // The virtual document key of the selection (root-prefixed in a multi-root
  // workspace); it is translated to this tree's root-relative space below.
  selectedPath: string | null;
  // Root identity: "" for a single-root preview (paths stay unprefixed) or
  // the root id whose virtual prefix this tree owns.
  rootBase?: string;
  // Present only in a multi-root workspace: renders the labeled root row the
  // tree nests under. Roots start expanded; collapsing uses the same
  // directory mechanics as inner directories.
  rootLabel?: string;
  // The root's canonical local path on the server and that machine's path
  // separator; together they build the absolute-path tooltips on directory
  // rows. Without an absolute path the rows simply carry no tooltip.
  rootAbsolutePath?: string;
  pathSeparator?: string;
  searching?: boolean;
  // False while the sidebar is collapsed offcanvas: geometry measured there is
  // meaningless, so the reveal waits until the tree becomes visible again.
  visible?: boolean;
  onSelect(path: string): void;
}

export function DocumentTree({
  files,
  selectedPath,
  rootBase = "",
  rootLabel,
  rootAbsolutePath,
  pathSeparator = "/",
  searching = false,
  visible = true,
  onSelect,
}: DocumentTreeProps) {
  const hasRootRow = rootLabel !== undefined;
  const tree = useMemo(() => buildTree(files), [files]);
  // Translate the virtual selection key into this root's path space: only the
  // tree owning the selected root reports a selection, every other root's
  // tree sees null.
  const selectedRelative =
    selectedPath !== null &&
    (rootBase === "" || selectedPath.startsWith(`${rootBase}/`))
      ? rootBase === ""
        ? selectedPath
        : selectedPath.slice(rootBase.length + 1)
      : null;
  const [expanded, setExpanded] = useState<Set<string>>(() => {
    const initial = new Set(ancestorDirectories(selectedRelative));
    if (hasRootRow) {
      initial.add(rootBase);
    }
    return initial;
  });
  const treeRef = useRef<HTMLUListElement>(null);
  const revealedPathRef = useRef<string | null>(null);

  useLayoutEffect(() => {
    const next = new Set(ancestorDirectories(selectedRelative));
    if (hasRootRow) {
      next.add(rootBase);
    }
    setExpanded((current) => {
      const merged = new Set(current);
      for (const ancestor of next) {
        merged.add(ancestor);
      }
      return merged;
    });
  }, [selectedRelative, hasRootRow, rootBase]);

  // Reveal the active file inside the sidebar's scroll viewport. Deliberately
  // only the viewport's own scrollTop is adjusted — scrollIntoView() could
  // scroll every scrollable ancestor up to the window and disturb the reader's
  // refresh position restore. The math mirrors block: "nearest": already
  // visible stays put, otherwise scroll just far enough to bring the item into
  // view, which is what file browsers do and avoids centering jumps. The
  // visible top starts below the active file's sticky ancestor rows: once the
  // reveal scrolls, those directories stick to the viewport top and would
  // otherwise cover the very item this just brought into view.
  // biome-ignore lint/correctness/useExhaustiveDependencies: expanded/tree/searching are deliberate re-run triggers — they reshape the rendered tree the DOM measurement below walks, even though the body never reads them.
  useLayoutEffect(() => {
    if (
      !visible ||
      selectedRelative === null ||
      revealedPathRef.current === selectedRelative
    ) {
      return;
    }

    const root = treeRef.current;
    if (root === null) {
      return;
    }

    const active = root.querySelector<HTMLElement>('[aria-current="page"]');
    const viewport = root.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );

    if (active === null || viewport === null) {
      return;
    }

    const viewportRect = viewport.getBoundingClientRect();
    const activeRect = active.getBoundingClientRect();
    // Measured from the DOM rather than derived from row-height constants so
    // density/font/accessibility scaling never desynchronizes the reservation.
    const ancestorPaths = new Set(ancestorDirectories(selectedRelative));
    if (hasRootRow) {
      ancestorPaths.add(rootBase);
    }
    const stickyHeight = Array.from(
      root.querySelectorAll<HTMLElement>('[data-tree-directory="true"]'),
    )
      .filter((element) => ancestorPaths.has(element.dataset.treePath ?? ""))
      .reduce(
        (height, element) => height + element.getBoundingClientRect().height,
        0,
      );
    const padding = 8;

    const visibleTop = viewportRect.top + stickyHeight + padding;
    const visibleBottom = viewportRect.bottom - padding;

    if (activeRect.top < visibleTop) {
      viewport.scrollTop -= visibleTop - activeRect.top;
    } else if (activeRect.bottom > visibleBottom) {
      viewport.scrollTop += activeRect.bottom - visibleBottom;
    }

    revealedPathRef.current = selectedRelative;
  }, [visible, selectedRelative, expanded, tree, searching]);

  // Collapsing a directory unmounts its whole subtree in one commit while the
  // viewport may be scrolled deep into it: scrollHeight/scrollWidth and the
  // sticky rows all mutate at once. This ref hands that mutation to the
  // normalization layout effect below, which re-clamps the viewport geometry
  // before the browser paints instead of trusting it to settle on its own.
  const collapsedPathRef = useRef<string | null>(null);

  // biome-ignore lint/correctness/useExhaustiveDependencies: expanded is the deliberate re-run trigger — the collapse has already removed the subtree by the time this commit runs, even though the body never reads it.
  useLayoutEffect(() => {
    if (collapsedPathRef.current === null) {
      return;
    }
    collapsedPathRef.current = null;

    const root = treeRef.current;
    const viewport = root?.closest<HTMLElement>(
      '[data-slot="scroll-area-viewport"]',
    );

    if (viewport === null || viewport === undefined) {
      return;
    }

    const maxScrollTop = Math.max(
      0,
      viewport.scrollHeight - viewport.clientHeight,
    );

    viewport.scrollTop = Math.max(
      0,
      Math.min(viewport.scrollTop, maxScrollTop),
    );
    viewport.scrollLeft = 0;
  }, [expanded]);

  const toggle = (path: string) => {
    const collapsing = expanded.has(path);

    if (collapsing) {
      collapsedPathRef.current = path;

      // Collapsing an ancestor of the selected file unmounts the active row,
      // so the reveal above bails without recording it. Clearing the recorded
      // path also covers the subtler case where the reveal already ran: on
      // re-expand, the active file is scrolled back into view instead of
      // staying wherever the collapse happened to leave the viewport.
      if (selectedRelative?.startsWith(`${path}/`)) {
        revealedPathRef.current = null;
      }
    }

    setExpanded((current) => {
      const next = new Set(current);

      if (next.has(path)) {
        next.delete(path);
      } else {
        next.add(path);
      }

      return next;
    });
  };

  // Selection identity crosses the tree boundary as the virtual key: inner
  // rows speak root-relative paths, the app addresses documents by
  // "<rootId>/<path>" in a multi-root workspace.
  const selectFromTree = (path: string) => {
    onSelect(rootBase === "" ? path : `${rootBase}/${path}`);
  };

  const items = (
    <>
      {tree.map((node) => (
        <TreeItem
          key={`${node.type}:${node.path}`}
          node={node}
          base={rootBase}
          rootAbsolutePath={rootAbsolutePath}
          pathSeparator={pathSeparator}
          expanded={expanded}
          selectedPath={selectedRelative}
          onSelect={selectFromTree}
          onToggle={toggle}
          searching={searching}
          depth={hasRootRow ? 1 : 0}
        />
      ))}
    </>
  );

  return (
    <SidebarMenu
      ref={treeRef}
      aria-label={
        rootLabel === undefined
          ? "Markdown 文件树"
          : `Markdown 文件树：${rootLabel}`
      }
    >
      {hasRootRow ? (
        <RootItem
          label={rootLabel}
          path={rootBase}
          absolutePath={rootAbsolutePath}
          expanded={searching || expanded.has(rootBase)}
          onToggle={toggle}
        >
          {items}
        </RootItem>
      ) : (
        items
      )}
    </SidebarMenu>
  );
}

// The labeled top-level row of one preview root in a multi-root workspace.
// It behaves exactly like a directory row — sticky, collapsible, chevron
// included — so the workspace reads as one tree of parallel roots.
function RootItem({
  label,
  path,
  absolutePath,
  expanded,
  onToggle,
  children,
}: {
  label: string;
  path: string;
  absolutePath?: string;
  expanded: boolean;
  onToggle(path: string): void;
  children: ReactNode;
}) {
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        aria-expanded={expanded}
        aria-label={label}
        data-tree-directory="true"
        data-tree-path={path}
        data-tree-depth={0}
        style={{ "--tree-sticky-top": "0rem" } as CSSProperties}
        className="document-tree-directory h-8 text-sm font-medium"
        onClick={() => onToggle(path)}
        tooltip={
          absolutePath === undefined
            ? undefined
            : directoryTooltip(absolutePath)
        }
      >
        <ChevronRight
          aria-hidden="true"
          className={
            expanded ? "rotate-90 transition-transform" : "transition-transform"
          }
        />
        {expanded ? (
          <FolderOpen aria-hidden="true" />
        ) : (
          <Folder aria-hidden="true" />
        )}
        <span className="truncate">{label}</span>
      </SidebarMenuButton>
      {expanded ? (
        <SidebarMenuSub className="document-tree-sub">
          {children}
        </SidebarMenuSub>
      ) : null}
    </SidebarMenuItem>
  );
}

// Directory rows (and the multi-root root row) reveal the server-local
// absolute path through the same Base UI tooltip the file rows already use.
// The wide panel lets a long path wrap instead of ellipsize — the path is the
// whole point of the tooltip. Replaces the rows' former native title
// attribute, which would double up with this tooltip.
function directoryTooltip(absolutePath: string) {
  return {
    hidden: false,
    side: "right" as const,
    align: "start" as const,
    className: "tree-tooltip tree-tooltip-wide",
    children: <span className="tree-tooltip-path">{absolutePath}</span>,
  };
}

interface TreeItemProps {
  node: TreeNode;
  // Root identity for accessible names: file rows announce their virtual
  // document key ("<rootId>/<path>") so same-named documents in two roots
  // stay distinguishable to assistive technology while the visible label
  // keeps the plain root-relative name.
  base: string;
  rootAbsolutePath?: string;
  pathSeparator: string;
  expanded: Set<string>;
  selectedPath: string | null;
  onSelect(path: string): void;
  onToggle(path: string): void;
  searching: boolean;
  depth: number;
}

function TreeItem({
  node,
  base,
  rootAbsolutePath,
  pathSeparator,
  expanded,
  selectedPath,
  onSelect,
  onToggle,
  searching,
  depth,
}: TreeItemProps) {
  if (node.type === "directory") {
    return (
      <DirectoryItem
        node={node}
        base={base}
        rootAbsolutePath={rootAbsolutePath}
        pathSeparator={pathSeparator}
        expanded={expanded}
        selectedPath={selectedPath}
        onSelect={onSelect}
        onToggle={onToggle}
        searching={searching}
        depth={depth}
      />
    );
  }
  const active = node.path === selectedPath;
  const identity = base === "" ? node.path : `${base}/${node.path}`;
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        isActive={active}
        aria-current={active ? "page" : undefined}
        aria-label={`${node.file.title}，${identity}`}
        className="document-tree-file h-8 text-sm"
        tooltip={{
          hidden: false,
          side: "right",
          align: "start",
          className: "tree-tooltip",
          children: (
            <>
              <span className="tree-tooltip-name">{node.name}</span>
              <span className="tree-tooltip-title">{node.file.title}</span>
            </>
          ),
        }}
        onClick={() => onSelect(node.path)}
      >
        <FileText aria-hidden="true" />
        <span className="truncate">{node.name}</span>
      </SidebarMenuButton>
    </SidebarMenuItem>
  );
}

function DirectoryItem({
  node,
  base,
  rootAbsolutePath,
  pathSeparator,
  expanded,
  selectedPath,
  onSelect,
  onToggle,
  searching,
  depth,
}: TreeItemProps & { node: DirectoryNode }) {
  const open = searching || expanded.has(node.path);
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        aria-expanded={open}
        aria-label={base === "" ? node.name : `${base}/${node.name}`}
        data-tree-directory="true"
        data-tree-path={node.path}
        data-tree-depth={depth}
        style={{ "--tree-sticky-top": `${depth * 2}rem` } as CSSProperties}
        className="document-tree-directory h-8 text-sm font-medium"
        onClick={() => onToggle(node.path)}
        tooltip={
          rootAbsolutePath === undefined
            ? undefined
            : directoryTooltip(
                localPath(rootAbsolutePath, node.path, pathSeparator),
              )
        }
      >
        <ChevronRight
          aria-hidden="true"
          className={
            open ? "rotate-90 transition-transform" : "transition-transform"
          }
        />
        {open ? (
          <FolderOpen aria-hidden="true" />
        ) : (
          <Folder aria-hidden="true" />
        )}
        <span className="truncate">{node.name}</span>
      </SidebarMenuButton>
      {open ? (
        <SidebarMenuSub className="document-tree-sub">
          {node.children.map((child) => (
            <TreeItem
              key={`${child.type}:${child.path}`}
              node={child}
              base={base}
              rootAbsolutePath={rootAbsolutePath}
              pathSeparator={pathSeparator}
              expanded={expanded}
              selectedPath={selectedPath}
              onSelect={onSelect}
              onToggle={onToggle}
              searching={searching}
              depth={depth + 1}
            />
          ))}
        </SidebarMenuSub>
      ) : null}
    </SidebarMenuItem>
  );
}
