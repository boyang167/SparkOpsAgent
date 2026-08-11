"use client";

import type { MenuTreeVM, RoleVM } from "@sparkops/contracts/governance";
import { MenuIconPreview } from "./menu-icons";
import { getMenuBreadcrumb } from "./menu-utils";

interface MenuDetailPanelProps {
  menu: MenuTreeVM | null;
  tree: MenuTreeVM[];
  roleIds: string[];
  roles: RoleVM[];
  loadingRoles: boolean;
  onEdit: (menu: MenuTreeVM) => void;
  onAssignRoles: (menu: MenuTreeVM) => void;
  onAddChild: (menu: MenuTreeVM) => void;
  onDelete: (menu: MenuTreeVM) => void;
}

export function MenuDetailPanel({
  menu,
  tree,
  roleIds,
  roles,
  loadingRoles,
  onEdit,
  onAssignRoles,
  onAddChild,
  onDelete,
}: MenuDetailPanelProps) {
  if (!menu) {
    return (
      <div className="menu-detail-empty">
        <div className="menu-detail-empty-icon" aria-hidden>
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
            <path d="M4 6h16M4 12h10M4 18h14" strokeLinecap="round" />
          </svg>
        </div>
        <h3 className="menu-detail-empty-title">选择菜单查看详情</h3>
        <p className="menu-detail-empty-desc">在左侧树中选择菜单项，可预览路由、授权角色并快速操作。</p>
      </div>
    );
  }

  const breadcrumb = getMenuBreadcrumb(tree, menu.id);
  const assignedRoles = roles.filter((r) => roleIds.includes(r.id));

  return (
    <div className="menu-detail-panel stack-md">
      <div className="menu-detail-header">
        <div className="menu-detail-icon-wrap">
          <MenuIconPreview icon={menu.icon} className="menu-detail-icon-lg" />
        </div>
        <div className="min-w-0 flex-1">
          <p className="menu-detail-breadcrumb">{breadcrumb.join(" / ")}</p>
          <h3 className="menu-detail-title">{menu.title}</h3>
          <p className="menu-detail-subtitle font-mono text-xs">{menu.path}</p>
        </div>
      </div>

      <div className="menu-detail-actions">
        <button type="button" className="btn-secondary !py-1.5 !px-3 text-xs" onClick={() => onEdit(menu)}>
          编辑菜单
        </button>
        <button type="button" className="btn-secondary !py-1.5 !px-3 text-xs" onClick={() => onAssignRoles(menu)}>
          分配角色
        </button>
        <button type="button" className="btn-secondary !py-1.5 !px-3 text-xs" onClick={() => onAddChild(menu)}>
          添加子菜单
        </button>
        <button type="button" className="btn-ghost !py-1.5 !px-3 text-xs text-red-600" onClick={() => onDelete(menu)}>
          删除
        </button>
      </div>

      <dl className="menu-detail-meta">
        <div className="menu-detail-meta-row">
          <dt>路由 Name</dt>
          <dd className="font-mono text-xs">{menu.name}</dd>
        </div>
        <div className="menu-detail-meta-row">
          <dt>路由 Path</dt>
          <dd className="font-mono text-xs">{menu.path}</dd>
        </div>
        <div className="menu-detail-meta-row">
          <dt>页面组件</dt>
          <dd className="font-mono text-xs break-all">{menu.component || "—"}</dd>
        </div>
        <div className="menu-detail-meta-row">
          <dt>排序</dt>
          <dd>{menu.sort}</dd>
        </div>
        <div className="menu-detail-meta-row">
          <dt>可见性</dt>
          <dd>
            {menu.hidden ? (
              <span className="badge badge-warning">隐藏</span>
            ) : (
              <span className="badge badge-success">显示</span>
            )}
          </dd>
        </div>
        <div className="menu-detail-meta-row">
          <dt>KeepAlive</dt>
          <dd>{menu.keep_alive ? "开启" : "关闭"}</dd>
        </div>
      </dl>

      <div className="menu-detail-section">
        <div className="menu-detail-section-head">
          <h4>已授权角色</h4>
          <span className="text-xs text-muted">{assignedRoles.length} 个</span>
        </div>
        {loadingRoles ? (
          <p className="text-sm text-muted">加载角色中…</p>
        ) : assignedRoles.length === 0 ? (
          <div className="menu-detail-alert">
            <p>尚未分配角色，该菜单不会对任何用户可见。</p>
            <button type="button" className="btn-secondary !py-1 !px-2 text-xs mt-2" onClick={() => onAssignRoles(menu)}>
              立即分配
            </button>
          </div>
        ) : (
          <div className="menu-detail-role-chips">
            {assignedRoles.map((role) => (
              <span key={role.id} className="badge badge-primary">
                {role.name}
                {role.name === "admin" ? " · 系统" : ""}
              </span>
            ))}
          </div>
        )}
      </div>

      <div className="menu-detail-preview">
        <h4>导航预览</h4>
        <div className="menu-detail-preview-nav">
          <MenuIconPreview icon={menu.icon} />
          <span>{menu.title}</span>
          {menu.hidden ? <span className="badge badge-warning ml-auto">隐藏</span> : null}
        </div>
      </div>
    </div>
  );
}
