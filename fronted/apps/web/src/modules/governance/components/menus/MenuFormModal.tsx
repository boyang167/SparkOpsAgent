"use client";

import type { RoleVM, UpsertMenuCommand } from "@sparkops/contracts/governance";
import { Modal } from "@/components/ui/Modal";
import { MenuAdvancedPanel } from "./MenuAdvancedPanel";
import { MENU_ICON_OPTIONS, MenuIconPreview } from "./menu-icons";

interface MenuFormModalProps {
  open: boolean;
  mode: "add" | "edit";
  form: UpsertMenuCommand;
  roles: RoleVM[];
  saving: boolean;
  detailLoading?: boolean;
  parentOptions: Array<{ id: string; label: string; depth: number }>;
  onClose: () => void;
  onChange: (next: UpsertMenuCommand) => void;
  onSubmit: (e: React.FormEvent) => void;
}

export function MenuFormModal({
  open,
  mode,
  form,
  roles,
  saving,
  detailLoading = false,
  parentOptions,
  onClose,
  onChange,
  onSubmit,
}: MenuFormModalProps) {
  const footer = (
    <div className="flex justify-end gap-2">
      <button type="button" className="btn-secondary" onClick={onClose}>
        取消
      </button>
      <button type="submit" form="menu-form" className="btn" disabled={saving || detailLoading}>
        {saving ? "保存中…" : mode === "add" ? "创建菜单" : "保存更改"}
      </button>
    </div>
  );

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={mode === "add" ? "新增菜单" : "编辑菜单"}
      description="配置菜单展示信息与路由，保存后可在右侧详情面板分配角色。"
      size="lg"
      priority
      footer={footer}
    >
      {detailLoading ? (
        <p className="text-sm text-muted mb-3" role="status">
          正在加载菜单详情…
        </p>
      ) : null}
      <form id="menu-form" onSubmit={onSubmit} className="stack-md">
        <div className="menu-form-grid">
          <section className="menu-form-section">
            <h3 className="menu-form-section-title">基础信息</h3>
            <label className="stack-xs">
              <span className="label">展示名称</span>
              <input
                className="input"
                value={form.title}
                onChange={(e) => onChange({ ...form, title: e.target.value })}
                placeholder="例如：构建管理"
                required
              />
            </label>
            <label className="stack-xs">
              <span className="label">路由 Name</span>
              <input
                className="input"
                value={form.name}
                onChange={(e) =>
                  onChange({
                    ...form,
                    name: e.target.value,
                    path: form.path || `/${e.target.value.replace(/\s+/g, "-").toLowerCase()}`,
                  })
                }
                placeholder="唯一标识，如 delivery-builds"
                required
              />
            </label>
            <label className="stack-xs">
              <span className="label">页面路径 / 组件</span>
              <input
                className="input"
                value={form.component}
                onChange={(e) => onChange({ ...form, component: e.target.value })}
                placeholder="例如 /delivery/builds"
              />
            </label>
          </section>

          <section className="menu-form-section">
            <h3 className="menu-form-section-title">路由与层级</h3>
            <label className="stack-xs">
              <span className="label">父级菜单</span>
              <select
                className="input"
                value={form.parent_id ?? ""}
                onChange={(e) =>
                  onChange({ ...form, parent_id: e.target.value ? e.target.value : null })
                }
              >
                <option value="">根目录（一级菜单）</option>
                {parentOptions.map((opt) => (
                  <option key={opt.id} value={opt.id}>
                    {"—".repeat(opt.depth)} {opt.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="stack-xs">
              <span className="label">路由 Path</span>
              <input
                className="input font-mono text-sm"
                value={form.path}
                onChange={(e) => onChange({ ...form, path: e.target.value })}
                placeholder="/delivery/builds"
                required
              />
            </label>
            <div className="menu-form-path-preview">
              <span className="text-xs text-muted">访问路径预览</span>
              <code>{form.path.startsWith("/") ? form.path : `/${form.path}`}</code>
            </div>
          </section>
        </div>

        <section className="menu-form-section">
          <h3 className="menu-form-section-title">显示与图标</h3>
          <div className="menu-icon-picker">
            {MENU_ICON_OPTIONS.map((opt) => (
              <button
                key={opt.value || "none"}
                type="button"
                className={`menu-icon-option ${form.icon === opt.value ? "menu-icon-option-active" : ""}`}
                onClick={() => onChange({ ...form, icon: opt.value })}
                title={opt.label}
              >
                <MenuIconPreview icon={opt.value || "default"} />
                <span>{opt.label}</span>
              </button>
            ))}
          </div>
          <div className="grid gap-3 sm:grid-cols-3 mt-3">
            <label className="stack-xs">
              <span className="label">排序权重</span>
              <input
                className="input"
                type="number"
                value={form.sort}
                onChange={(e) => onChange({ ...form, sort: Number(e.target.value) || 0 })}
              />
            </label>
            <label className="stack-xs">
              <span className="label">菜单可见性</span>
              <select
                className="input"
                value={form.hidden ? "1" : "0"}
                onChange={(e) => onChange({ ...form, hidden: e.target.value === "1" })}
              >
                <option value="0">显示在导航</option>
                <option value="1">隐藏（仅授权可访问）</option>
              </select>
            </label>
          </div>
        </section>

        <MenuAdvancedPanel form={form} roles={roles} onChange={onChange} />
      </form>
    </Modal>
  );
}
