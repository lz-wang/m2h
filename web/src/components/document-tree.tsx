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
  // view, which is what file browsers do and avoids centering jumps.
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
    const padding = 8;

    const visibleTop = viewportRect.top + padding;
    const visibleBottom = viewportRect.bottom - padding;

    if (activeRect.top < visibleTop) {
      viewport.scrollTop -= visibleTop - activeRect.top;
    } else if (activeRect.bottom > visibleBottom) {
      viewport.scrollTop += activeRect.bottom - visibleBottom;
    }

    revealedPathRef.current = selectedPath;
  }, [visible, selectedPath, expanded, tree, searching]);

  const toggle = (path: string) => {
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
        <SidebarMenuSub>
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
