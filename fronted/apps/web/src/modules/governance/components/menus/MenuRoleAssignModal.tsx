"use client";

import { useMemo, useState } from "react";
import type { RoleVM } from "@sparkops/contracts/governance";
import { Modal } from "@/components/ui/Modal";

interface MenuRoleAssignModalProps {
  open: boolean;
  menuTitle: string;
  roles: RoleVM[];
  selectedRoleIds: string[];
  loading: boolean;
  onClose: () => void;
  onToggleRole: (roleId: string) => void;
  onSetRoleIds: (roleIds: string[]) => void;
  onSubmit: (e: React.FormEvent) => void;
}

export function MenuRoleAssignModal({
  open,
  menuTitle,
  roles,
  selectedRoleIds,
  loading,
  onClose,
  onToggleRole,
  onSetRoleIds,
  onSubmit,
}: MenuRoleAssignModalProps) {
  const [query, setQuery] = useState("");

  const filteredRoles = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return roles;
    return roles.filter((r) => r.name.toLowerCase().includes(q));
  }, [roles, query]);

  const footer = (
    <div className="flex flex-wrap items-center justify-between gap-2 w-full">
      <span className="text-xs text-muted">已选 {selectedRoleIds.length} / {roles.length} 个角色</span>
      <div className="flex gap-2">
        <button type="button" className="btn-secondary" onClick={onClose}>
          取消
        </button>
        <button type="submit" form="assign-roles-form" className="btn" disabled={loading}>
          {loading ? "保存中…" : "保存授权"}
        </button>
      </div>
    </div>
  );

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={`分配角色 · ${menuTitle}`}
      description="勾选可访问该菜单的角色。保存后将全量覆盖现有关联。"
      size="lg"
      footer={footer}
    >
      <form id="assign-roles-form" onSubmit={onSubmit} className="stack-md">
        <div className="menu-role-toolbar">
          <input
            className="input"
            placeholder="搜索角色名称…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <button
            type="button"
            className="btn-secondary !py-1.5 !px-3 text-xs whitespace-nowrap"
            onClick={() => onSetRoleIds(roles.map((r) => r.id))}
          >
            全选
          </button>
          <button
            type="button"
            className="btn-secondary !py-1.5 !px-3 text-xs whitespace-nowrap"
            onClick={() => onSetRoleIds([])}
          >
            清空
          </button>
        </div>

        {loading && roles.length === 0 ? (
          <div className="empty-state">加载中…</div>
        ) : filteredRoles.length === 0 ? (
          <div className="empty-state">没有匹配的角色</div>
        ) : (
          <div className="menu-role-grid">
            {filteredRoles.map((role) => {
              const checked = selectedRoleIds.includes(role.id);
              return (
                <label
                  key={role.id}
                  className={`menu-role-card ${checked ? "menu-role-card-selected" : ""}`}
                >
                  <input
                    type="checkbox"
                    className="menu-role-checkbox"
                    checked={checked}
                    onChange={() => onToggleRole(role.id)}
                  />
                  <div className="menu-role-card-body">
                    <div className="flex items-center gap-2">
                      <span className="font-medium text-sm">{role.name}</span>
                      {role.name === "admin" ? <span className="badge badge-neutral">系统</span> : null}
                    </div>
                    <p className="text-xs text-muted mt-1 line-clamp-2">
                      {role.permissions.slice(0, 3).join(" · ")}
                      {role.permissions.length > 3 ? ` 等 ${role.permissions.length} 项权限` : ""}
                    </p>
                  </div>
                </label>
              );
            })}
          </div>
        )}
      </form>
    </Modal>
  );
}
