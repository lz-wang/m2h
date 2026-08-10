import { ChevronRight, FileText, Folder, FolderOpen } from "lucide-react";
import { type RefObject, useEffect, useMemo, useRef, useState } from "react";
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
    <SidebarMenu aria-label="Markdown 文件树">
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
    </SidebarMenu>
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
    <SidebarMenuItem>
      <SidebarMenuButton
        ref={active ? activeItem : undefined}
        isActive={active}
        aria-current={active ? "page" : undefined}
        aria-label={`${node.file.title}，${node.path}`}
        className="h-8 text-sm"
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
  activeItem,
}: TreeItemProps & { node: DirectoryNode }) {
  const open = expanded.has(node.path);
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        aria-expanded={open}
        aria-label={node.name}
        className="h-8 text-sm font-medium"
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
              activeItem={activeItem}
            />
          ))}
        </SidebarMenuSub>
      ) : null}
    </SidebarMenuItem>
  );
}
