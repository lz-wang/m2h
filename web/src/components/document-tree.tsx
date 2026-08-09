import { ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import { type RefObject, useEffect, useMemo, useRef, useState } from "react";
import type { FileSummary } from "@/api";
import {
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
  onSelect(path: string): void;
}

export function DocumentTree({
  files,
  selectedPath,
  onSelect,
}: DocumentTreeProps) {
  const tree = useMemo(() => buildTree(files), [files]);
  const [expanded, setExpanded] = useState<Set<string>>(
    () => new Set(ancestorDirectories(selectedPath)),
  );
  const activeItem = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    const ancestors = ancestorDirectories(selectedPath);
    setExpanded((current) => {
      const next = new Set(current);
      for (const ancestor of ancestors) {
        next.add(ancestor);
      }
      return next;
    });
    const frame = window.requestAnimationFrame(() => {
      activeItem.current?.scrollIntoView?.({ block: "nearest" });
    });
    return () => window.cancelAnimationFrame(frame);
  }, [selectedPath]);

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
    <ul aria-label="Markdown 文件树" className="document-tree">
      {tree.map((node) => (
        <TreeItem
          key={`${node.type}:${node.path}`}
          node={node}
          expanded={expanded}
          selectedPath={selectedPath}
          onSelect={onSelect}
          onToggle={toggle}
          activeItem={activeItem}
        />
      ))}
    </ul>
  );
}

interface TreeItemProps {
  node: TreeNode;
  expanded: Set<string>;
  selectedPath: string | null;
  onSelect(path: string): void;
  onToggle(path: string): void;
  activeItem: RefObject<HTMLButtonElement | null>;
}

function TreeItem({
  node,
  expanded,
  selectedPath,
  onSelect,
  onToggle,
  activeItem,
}: TreeItemProps) {
  if (node.type === "directory") {
    return (
      <DirectoryItem
        node={node}
        expanded={expanded}
        selectedPath={selectedPath}
        onSelect={onSelect}
        onToggle={onToggle}
        activeItem={activeItem}
      />
    );
  }
  const active = node.path === selectedPath;
  return (
    <SidebarMenuItem className="tree-file-item">
      <SidebarMenuButton
        ref={active ? activeItem : undefined}
        isActive={active}
        aria-current={active ? "page" : undefined}
        aria-label={`${node.file.title}，${node.path}`}
        className="tree-file-button"
        onClick={() => onSelect(node.path)}
      >
        <FileText aria-hidden="true" />
        <span>{node.name}</span>
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
  activeItem,
}: TreeItemProps & { node: DirectoryNode }) {
  const open = expanded.has(node.path);
  return (
    <li className="tree-directory-item">
      <button
        type="button"
        className="tree-directory-button"
        aria-expanded={open}
        onClick={() => onToggle(node.path)}
      >
        <ChevronRight
          aria-hidden="true"
          className={open ? "tree-chevron is-open" : "tree-chevron"}
        />
        {open ? (
          <FolderOpen aria-hidden="true" />
        ) : (
          <Folder aria-hidden="true" />
        )}
        <span>{node.name}</span>
      </button>
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
              activeItem={activeItem}
            />
          ))}
        </SidebarMenuSub>
      ) : null}
    </li>
  );
}
