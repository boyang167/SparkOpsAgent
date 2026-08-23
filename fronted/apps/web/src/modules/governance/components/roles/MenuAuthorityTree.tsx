"use client";

import { useMemo, useState } from "react";
import type { MenuTreeVM } from "@sparkops/contracts/governance";
import {
  menuSelectionState,
  setAllMenuSelection,
  toggleMenuSelection,
} from "./menu-authority-utils";
import { collectExpandableIds } from "../menus/menu-utils";

interface MenuAuthorityTreeProps {
  tree: MenuTreeVM[];
  selected: Set<string>;
  onChange: (next: Set<string>) => void;
  query?: string;
}

function MenuAuthorityNode({
  node,
  depth,
  tree,
  selected,
  onChange,
  expanded,
  onToggleExpand,
}: {
  node: MenuTreeVM;
  depth: number;
  tree: MenuTreeVM[];
  selected: Set<string>;
  onChange: (next: Set<string>) => void;
  expanded: Set<string>;
  onToggleExpand: (id: string) => void;
}) {
  const state = menuSelectionState(tree, selected, node.id);
  const hasChildren = Boolean(node.children?.length);

  return (
    <>
      <div className="role-menu-tree-row" style={{ paddingLeft: `${depth * 1.125 + 0.5}rem` }}>
        {hasChildren ? (
          <button
            type="button"
            className="role-menu-tree-expand"
            aria-label={expanded.has(node.id) ? "收起" : "展开"}
            onClick={() => onToggleExpand(node.id)}
          >
            {expanded.has(node.id) ? "▾" : "▸"}
          </button>
        ) : (
          <span className="role-menu-tree-expand role-menu-tree-expand-placeholder" />
        )}
        <label className="role-menu-tree-label">
          <input
            type="checkbox"
            checked={state === "checked"}
            ref={(el) => {
              if (el) el.indeterminate = state === "indeterminate";
            }}
            onChange={(e) => onChange(toggleMenuSelection(tree, selected, node.id, e.target.checked))}
          />
          <span className="role-menu-tree-title">{node.title}</span>
          <span className="role-menu-tree-path">{node.path}</span>
          {node.hidden ? <span className="badge badge-neutral">隐藏</span> : null}
        </label>
      </div>
      {hasChildren && expanded.has(node.id)
        ? node.children!.map((child) => (
            <MenuAuthorityNode
              key={child.id}
              node={child}
              depth={depth + 1}
              tree={tree}
              selected={selected}
              onChange={onChange}
              expanded={expanded}
              onToggleExpand={onToggleExpand}
            />
          ))
        : null}
    </>
  );
}

export function MenuAuthorityTree({ tree, selected, onChange, query = "" }: MenuAuthorityTreeProps) {
  const [expanded, setExpanded] = useState<Set<string>>(() => new Set(collectExpandableIds(tree)));

  const filteredTree = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return tree;
    function walk(items: MenuTreeVM[]): MenuTreeVM[] {
      const out: MenuTreeVM[] = [];
      for (const item of items) {
        const children = item.children?.length ? walk(item.children) : [];
        const matched =
          item.title.toLowerCase().includes(q) ||
          item.name.toLowerCase().includes(q) ||
          item.path.toLowerCase().includes(q);
        if (matched || children.length > 0) {
          out.push({ ...item, children: children.length ? children : undefined });
        }
      }
      return out;
    }
    return walk(tree);
  }, [tree, query]);

  function toggleExpand(id: string) {
    setExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }

  if (filteredTree.length === 0) {
    return <div className="empty-state">暂无匹配菜单</div>;
  }

  return (
    <div className="role-menu-tree">
      <div className="role-menu-tree-toolbar">
        <button type="button" className="btn-secondary !py-1 !px-2 text-xs" onClick={() => onChange(setAllMenuSelection(tree, true))}>
          全选
        </button>
        <button type="button" className="btn-secondary !py-1 !px-2 text-xs" onClick={() => onChange(new Set())}>
          清空
        </button>
        <span className="text-xs text-muted ml-auto">已选 {selected.size} 项</span>
      </div>
      <div className="role-menu-tree-body">
        {filteredTree.map((node) => (
          <MenuAuthorityNode
            key={node.id}
            node={node}
            depth={0}
            tree={tree}
            selected={selected}
            onChange={onChange}
            expanded={expanded}
            onToggleExpand={toggleExpand}
          />
        ))}
      </div>
    </div>
  );
}
