import { ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import type { CSSProperties } from "react";
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

interface DocumentTreeProps {
  files: FileSummary[];
  selectedPath: string | null;
  searching?: boolean;
  // False while the sidebar is collapsed offcanvas: geometry measured there is
  // meaningless, so the reveal waits until the tree becomes visible again.
  visible?: boolean;
  onSelect(path: string): void;
}

export function DocumentTree({
  files,
  selectedPath,
  searching = false,
  visible = true,
  onSelect,
}: DocumentTreeProps) {
  const tree = useMemo(() => buildTree(files), [files]);
  const [expanded, setExpanded] = useState<Set<string>>(
    () => new Set(ancestorDirectories(selectedPath)),
  );
  const treeRef = useRef<HTMLUListElement>(null);
  const revealedPathRef = useRef<string | null>(null);

  useLayoutEffect(() => {
    const ancestors = ancestorDirectories(selectedPath);
    setExpanded((current) => {
      const next = new Set(current);
      for (const ancestor of ancestors) {
        next.add(ancestor);
      }
      return next;
    });
  }, [selectedPath]);

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
      selectedPath === null ||
      revealedPathRef.current === selectedPath
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
    const ancestorPaths = new Set(ancestorDirectories(selectedPath));
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

    revealedPathRef.current = selectedPath;
  }, [visible, selectedPath, expanded, tree, searching]);

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
      if (selectedPath?.startsWith(`${path}/`)) {
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

  return (
    <SidebarMenu ref={treeRef} aria-label="Markdown 文件树">
      {tree.map((node) => (
        <TreeItem
          key={`${node.type}:${node.path}`}
          node={node}
          expanded={expanded}
          selectedPath={selectedPath}
          onSelect={onSelect}
          onToggle={toggle}
          searching={searching}
          depth={0}
        />
      ))}
    </SidebarMenu>
  );
}

interface TreeItemProps {
  node: TreeNode;
  expanded: Set<string>;
  selectedPath: string | null;
  onSelect(path: string): void;
  onToggle(path: string): void;
  searching: boolean;
  depth: number;
}

function TreeItem({
  node,
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
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        isActive={active}
        aria-current={active ? "page" : undefined}
        aria-label={`${node.file.title}，${node.path}`}
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
        aria-label={node.name}
        title={node.name}
        data-tree-directory="true"
        data-tree-path={node.path}
        data-tree-depth={depth}
        style={{ "--tree-sticky-top": `${depth * 2}rem` } as CSSProperties}
        className="document-tree-directory h-8 text-sm font-medium"
        onClick={() => onToggle(node.path)}
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
