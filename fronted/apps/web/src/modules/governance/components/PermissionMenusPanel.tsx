"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { MenuTreeVM, RoleVM, UpsertMenuCommand } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { MetricCard } from "@/components/ui/MetricCard";
import { MenuDetailPanel } from "./menus/MenuDetailPanel";
import { MenuFormModal } from "./menus/MenuFormModal";
import { MenuRoleAssignModal } from "./menus/MenuRoleAssignModal";
import { MenuTreeView } from "./menus/MenuTreeView";
import {
  collectExpandableIds,
  collectMenuIds,
  collectParentOptions,
  computeMenuStats,
  filterMenuTree,
  findMenuNode,
  menuTreeToForm,
  notifyMenusUpdated,
} from "./menus/menu-utils";

interface PermissionMenusPanelProps {
  workspaceId: string;
}

const EMPTY_FORM: UpsertMenuCommand = {
  parent_id: null,
  name: "",
  path: "",
  title: "",
  icon: "",
  component: "",
  hidden: false,
  sort: 0,
  keep_alive: false,
  active_menu: "",
  close_tab: false,
  is_basic: false,
  transition_animation: "global",
  params: [],
  buttons: [],
};

export function PermissionMenusPanel({ workspaceId }: PermissionMenusPanelProps) {
  const [tree, setTree] = useState<MenuTreeVM[]>([]);
  const [roles, setRoles] = useState<RoleVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);
  const [actionId, setActionId] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [showHidden, setShowHidden] = useState(true);
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());
  const [selectedId, setSelectedId] = useState<string | null>(null);

  const [roleCounts, setRoleCounts] = useState<Record<string, number>>({});
  const [selectedRoleIds, setSelectedRoleIds] = useState<string[]>([]);
  const [detailRoleIds, setDetailRoleIds] = useState<string[]>([]);
  const [detailRolesLoading, setDetailRolesLoading] = useState(false);

  const [formOpen, setFormOpen] = useState(false);
  const [formMode, setFormMode] = useState<"add" | "edit">("add");
  const [editingId, setEditingId] = useState<string | null>(null);
  const [form, setForm] = useState<UpsertMenuCommand>(EMPTY_FORM);
  const [formDetailLoading, setFormDetailLoading] = useState(false);
  const [saving, setSaving] = useState(false);

  const [assignMenu, setAssignMenu] = useState<MenuTreeVM | null>(null);
  const [assignLoading, setAssignLoading] = useState(false);

  const [deleteTarget, setDeleteTarget] = useState<MenuTreeVM | null>(null);

  const stats = useMemo(() => computeMenuStats(tree), [tree]);
  const filteredTree = useMemo(
    () => filterMenuTree(tree, search, showHidden),
    [tree, search, showHidden],
  );
  const parentOptions = useMemo(
    () => collectParentOptions(tree, formMode === "edit" ? editingId ?? undefined : undefined),
    [tree, formMode, editingId],
  );
  const selectedMenu = useMemo(
    () => (selectedId ? findMenuNode(tree, selectedId) : null),
    [tree, selectedId],
  );

  const loadRoleCounts = useCallback(async (menuTree: MenuTreeVM[]) => {
    const ids = collectMenuIds(menuTree);
    if (ids.length === 0) {
      setRoleCounts({});
      return;
    }
    const entries = await Promise.all(
      ids.map(async (id) => {
        try {
          const roleIds = await governance.getMenuRoles(workspaceId, id);
          return [id, roleIds.length] as const;
        } catch {
          return [id, 0] as const;
        }
      }),
    );
    setRoleCounts(Object.fromEntries(entries));
  }, [workspaceId]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [menuTree, roleList] = await Promise.all([
        governance.listMenuTree(workspaceId),
        governance.listRoles(workspaceId),
      ]);
      setTree(menuTree);
      setRoles(roleList);
      setExpandedIds(new Set(collectExpandableIds(menuTree)));
      setSelectedId((current) => (current && !findMenuNode(menuTree, current) ? null : current));
      void loadRoleCounts(menuTree);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, loadRoleCounts]);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (!selectedId) {
      setDetailRoleIds([]);
      return;
    }
    let cancelled = false;
    setDetailRolesLoading(true);
    governance
      .getMenuRoles(workspaceId, selectedId)
      .then((ids) => {
        if (!cancelled) setDetailRoleIds(ids);
      })
      .catch(() => {
        if (!cancelled) setDetailRoleIds([]);
      })
      .finally(() => {
        if (!cancelled) setDetailRolesLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [workspaceId, selectedId]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 3200);
  }

  function openCreate(parentId?: string | null) {
    setFormMode("add");
    setEditingId(null);
    setForm({ ...EMPTY_FORM, parent_id: parentId ?? null });
    setFormOpen(true);
  }

  async function openEdit(menu: MenuTreeVM) {
    if (!menu.id) {
      setError("菜单 ID 无效，无法编辑");
      return;
    }
    setFormMode("edit");
    setEditingId(menu.id);
    setError(null);
    setForm(menuTreeToForm(menu));
    setFormOpen(true);
    setFormDetailLoading(true);
    try {
      const detail = await governance.getMenu(workspaceId, menu.id);
      setForm({
        parent_id: detail.parent_id ?? null,
        name: detail.name,
        path: detail.path,
        title: detail.title,
        icon: detail.icon ?? "",
        component: detail.component ?? "",
        hidden: detail.hidden,
        sort: detail.sort,
        keep_alive: detail.keep_alive,
        active_menu: detail.active_menu ?? "",
        close_tab: detail.close_tab ?? false,
        is_basic: detail.is_basic ?? false,
        transition_animation:
          detail.transition_animation === "fade" || detail.transition_animation === "none"
            ? detail.transition_animation
            : "global",
        params: detail.params ?? [],
        buttons: detail.buttons ?? [],
      });
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载菜单详情失败，仍可使用基础字段编辑");
    } finally {
      setFormDetailLoading(false);
    }
  }

  async function onSaveForm(e: React.FormEvent) {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      if (formMode === "add") {
        await governance.createMenu(workspaceId, form);
        flashSuccess("菜单已创建，请为角色分配访问权限");
      } else if (editingId) {
        await governance.updateMenu(workspaceId, editingId, form);
        flashSuccess("菜单已更新");
      }
      notifyMenusUpdated();
      setFormOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  async function confirmDelete() {
    if (!deleteTarget) return;
    setActionId(deleteTarget.id);
    setError(null);
    try {
      await governance.deleteMenu(workspaceId, deleteTarget.id);
      if (selectedId === deleteTarget.id) setSelectedId(null);
      flashSuccess("菜单已删除");
      notifyMenusUpdated();
      setDeleteTarget(null);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setActionId(null);
    }
  }

  async function openAssignRoles(menu: MenuTreeVM) {
    setAssignMenu(menu);
    setAssignLoading(true);
    setError(null);
    try {
      setSelectedRoleIds(await governance.getMenuRoles(workspaceId, menu.id));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载角色失败");
      setAssignMenu(null);
    } finally {
      setAssignLoading(false);
    }
  }

  async function onConfirmAssignRoles(e: React.FormEvent) {
    e.preventDefault();
    if (!assignMenu) return;
    setAssignLoading(true);
    setError(null);
    try {
      await governance.setMenuRoles(workspaceId, assignMenu.id, { role_ids: selectedRoleIds });
      flashSuccess("角色授权已保存");
      notifyMenusUpdated();
      setAssignMenu(null);
      if (selectedId === assignMenu.id) {
        setDetailRoleIds(selectedRoleIds);
      }
      await loadRoleCounts(tree);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "分配失败");
    } finally {
      setAssignLoading(false);
    }
  }

  function toggleExpand(menuId: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(menuId)) next.delete(menuId);
      else next.add(menuId);
      return next;
    });
  }

  function expandAll() {
    setExpandedIds(new Set(collectExpandableIds(filteredTree)));
  }

  function collapseAll() {
    setExpandedIds(new Set());
  }

  return (
    <div className="stack-lg">
      <div className="menu-admin-toolbar-top">
        <p className="text-sm text-muted">
          配置工作区侧边栏菜单树、路由与角色授权。用户仅能看到其角色已授权的菜单项。
        </p>
        <div className="flex flex-wrap items-center gap-2">
          <button type="button" className="btn-secondary" onClick={() => void load()} disabled={loading}>
            刷新
          </button>
          <button type="button" className="btn" onClick={() => openCreate(null)}>
            新增根菜单
          </button>
        </div>
      </div>

      <div className="menu-admin-stats">
        <MetricCard
          label="菜单总数"
          value={String(stats.total)}
          hint={`${stats.roots} 个一级分组`}
          tone="sky"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M4 6h16M4 12h16M4 18h10" strokeLinecap="round" />
            </svg>
          }
        />
        <MetricCard
          label="叶子菜单"
          value={String(stats.leaves)}
          hint="可直达页面的菜单项"
          tone="emerald"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M12 3v18M3 12h18" strokeLinecap="round" />
            </svg>
          }
        />
        <MetricCard
          label="隐藏菜单"
          value={String(stats.hidden)}
          hint="不在导航中展示"
          tone="amber"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M3 12s3.5-7 9-7 9 7 9 7-3.5 7-9 7-9-7-9-7z" />
              <circle cx="12" cy="12" r="2.5" />
              <path d="M3 3l18 18" strokeLinecap="round" />
            </svg>
          }
        />
        <MetricCard
          label="工作区角色"
          value={String(roles.length)}
          hint="可参与菜单授权"
          tone="violet"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M12 3l7 4v5c0 4.4-3 8.5-7 9-4-.5-7-4.6-7-9V7l7-4z" strokeLinejoin="round" />
            </svg>
          }
        />
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}

      <div className="menu-admin-layout">
        <div className="menu-admin-main card">
          <div className="menu-admin-toolbar">
            <div className="menu-admin-search">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
                <circle cx="11" cy="11" r="7" />
                <path d="M20 20l-3.5-3.5" strokeLinecap="round" />
              </svg>
              <input
                className="input menu-admin-search-input"
                placeholder="搜索菜单名称、路由或组件路径…"
                value={search}
                onChange={(e) => setSearch(e.target.value)}
              />
            </div>
            <label className="menu-admin-filter">
              <input
                type="checkbox"
                checked={showHidden}
                onChange={(e) => setShowHidden(e.target.checked)}
              />
              显示隐藏项
            </label>
            <button type="button" className="btn-ghost text-xs" onClick={expandAll}>
              全部展开
            </button>
            <button type="button" className="btn-ghost text-xs" onClick={collapseAll}>
              全部收起
            </button>
          </div>

          {loading ? (
            <div className="menu-admin-skeleton">
              {Array.from({ length: 6 }).map((_, i) => (
                <div key={i} className="menu-admin-skeleton-row" style={{ width: `${70 - i * 6}%` }} />
              ))}
            </div>
          ) : filteredTree.length === 0 ? (
            <div className="menu-admin-empty">
              <p>{search ? "没有匹配的菜单" : "暂无菜单数据"}</p>
              {!search ? (
                <button type="button" className="btn mt-3" onClick={() => openCreate(null)}>
                  创建第一个菜单
                </button>
              ) : null}
            </div>
          ) : (
            <MenuTreeView
              nodes={filteredTree}
              selectedId={selectedId}
              expandedIds={expandedIds}
              roleCounts={roleCounts}
              actionId={actionId}
              onSelect={(menu) => setSelectedId(menu.id)}
              onToggleExpand={toggleExpand}
              onAddChild={(menu) => openCreate(menu.id)}
              onEdit={(menu) => void openEdit(menu)}
              onAssignRoles={(menu) => void openAssignRoles(menu)}
              onDelete={(menu) => setDeleteTarget(menu)}
            />
          )}
        </div>

        <aside className="menu-admin-detail card">
          <MenuDetailPanel
            menu={selectedMenu}
            tree={tree}
            roleIds={detailRoleIds}
            roles={roles}
            loadingRoles={detailRolesLoading}
            onEdit={(menu) => void openEdit(menu)}
            onAssignRoles={(menu) => void openAssignRoles(menu)}
            onAddChild={(menu) => openCreate(menu.id)}
            onDelete={(menu) => setDeleteTarget(menu)}
          />
        </aside>
      </div>

      <MenuFormModal
        open={formOpen}
        mode={formMode}
        form={form}
        roles={roles}
        saving={saving}
        detailLoading={formDetailLoading}
        parentOptions={parentOptions}
        onClose={() => {
          setFormOpen(false);
          setFormDetailLoading(false);
        }}
        onChange={setForm}
        onSubmit={onSaveForm}
      />

      <MenuRoleAssignModal
        open={Boolean(assignMenu)}
        menuTitle={assignMenu?.title ?? ""}
        roles={roles}
        selectedRoleIds={selectedRoleIds}
        loading={assignLoading}
        onClose={() => setAssignMenu(null)}
        onToggleRole={(roleId) =>
          setSelectedRoleIds((prev) =>
            prev.includes(roleId) ? prev.filter((id) => id !== roleId) : [...prev, roleId],
          )
        }
        onSetRoleIds={setSelectedRoleIds}
        onSubmit={onConfirmAssignRoles}
      />

      <Modal
        open={Boolean(deleteTarget)}
        onClose={() => setDeleteTarget(null)}
        title="确认删除菜单"
        description={
          deleteTarget
            ? `将删除「${deleteTarget.title}」及其所有子菜单，并解除相关角色授权。此操作不可撤销。`
            : undefined
        }
        footer={
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setDeleteTarget(null)}>
              取消
            </button>
            <button
              type="button"
              className="btn !bg-red-600 hover:!bg-red-700"
              disabled={Boolean(actionId)}
              onClick={() => void confirmDelete()}
            >
              {actionId ? "删除中…" : "确认删除"}
            </button>
          </div>
        }
      >
        <p className="text-sm text-muted">
          删除后，已授权该菜单的用户将无法再从侧边栏看到对应入口。如需临时下线，建议改为「隐藏」而非删除。
        </p>
      </Modal>
    </div>
  );
}
