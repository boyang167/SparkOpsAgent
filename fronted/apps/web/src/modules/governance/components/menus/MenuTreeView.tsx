"use client";

import type { MenuTreeVM } from "@sparkops/contracts/governance";
import { MenuIconPreview } from "./menu-icons";

interface MenuTreeViewProps {
  nodes: MenuTreeVM[];
  depth?: number;
  selectedId: string | null;
  expandedIds: Set<string>;
  roleCounts: Record<string, number>;
  actionId: string | null;
  onSelect: (menu: MenuTreeVM) => void;
  onToggleExpand: (menuId: string) => void;
  onAddChild: (menu: MenuTreeVM) => void;
  onEdit: (menu: MenuTreeVM) => void;
  onAssignRoles: (menu: MenuTreeVM) => void;
  onDelete: (menu: MenuTreeVM) => void;
}

export function MenuTreeView({
  nodes,
  depth = 0,
  selectedId,
  expandedIds,
  roleCounts,
  actionId,
  onSelect,
  onToggleExpand,
  onAddChild,
  onEdit,
  onAssignRoles,
  onDelete,
}: MenuTreeViewProps) {
  return (
    <ul className="menu-tree-list" role="tree">
      {nodes.map((node) => {
        const hasChildren = Boolean(node.children?.length);
        const expanded = expandedIds.has(node.id);
        const selected = selectedId === node.id;
        const roleCount = roleCounts[node.id] ?? 0;

        return (
          <li key={node.id} className="menu-tree-item" role="treeitem" aria-expanded={hasChildren ? expanded : undefined}>
            <div
              className={`menu-tree-row ${selected ? "menu-tree-row-selected" : ""}`}
              style={{ paddingLeft: `${depth * 1.25 + 0.5}rem` }}
            >
              <button
                type="button"
                className={`menu-tree-expander ${hasChildren ? "" : "menu-tree-expander-placeholder"}`}
                aria-label={expanded ? "收起" : "展开"}
                disabled={!hasChildren}
                onClick={() => hasChildren && onToggleExpand(node.id)}
              >
                {hasChildren ? (
                  <svg
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    className={expanded ? "menu-tree-chevron-open" : "menu-tree-chevron"}
                  >
                    <path d="M9 6l6 6-6 6" strokeLinecap="round" strokeLinejoin="round" />
                  </svg>
                ) : null}
              </button>

              <button type="button" className="menu-tree-main" onClick={() => onSelect(node)}>
                <MenuIconPreview icon={node.icon} />
                <span className="menu-tree-title">{node.title}</span>
                {node.hidden ? <span className="badge badge-warning menu-tree-badge">隐藏</span> : null}
                {roleCount > 0 ? (
                  <span className="badge badge-neutral menu-tree-badge">{roleCount} 角色</span>
                ) : (
                  <span className="badge menu-tree-badge menu-tree-badge-muted">未授权</span>
                )}
              </button>

              <div className="menu-tree-actions">
                <button
                  type="button"
                  className="menu-tree-action"
                  title="添加子菜单"
                  onClick={() => onAddChild(node)}
                >
                  子项
                </button>
                <button type="button" className="menu-tree-action" title="编辑" onClick={() => onEdit(node)}>
                  编辑
                </button>
                <button
                  type="button"
                  className="menu-tree-action"
                  title="分配角色"
                  onClick={() => onAssignRoles(node)}
                >
                  角色
                </button>
                <button
                  type="button"
                  className="menu-tree-action menu-tree-action-danger"
                  title="删除"
                  disabled={actionId === node.id}
                  onClick={() => onDelete(node)}
                >
                  删除
                </button>
              </div>
            </div>

            {hasChildren && expanded ? (
              <MenuTreeView
                nodes={node.children!}
                depth={depth + 1}
                selectedId={selectedId}
                expandedIds={expandedIds}
                roleCounts={roleCounts}
                actionId={actionId}
                onSelect={onSelect}
                onToggleExpand={onToggleExpand}
                onAddChild={onAddChild}
                onEdit={onEdit}
                onAssignRoles={onAssignRoles}
                onDelete={onDelete}
              />
            ) : null}
          </li>
        );
      })}
    </ul>
  );
}
