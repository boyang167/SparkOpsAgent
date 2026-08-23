"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";
import {
  DATA_ACTIONS,
  DATA_SCOPES,
  type DataAction,
  type DataResourceVM,
  type MenuTreeVM,
  type RegisteredAPI,
  type RoleAPIPolicy,
  type RoleDataPolicyVM,
  type RoleTreeVM,
  type WorkspaceMenuButtonVM,
} from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Drawer } from "@/components/ui/Drawer";
import { MenuAuthorityTree } from "./MenuAuthorityTree";
import { collectRoleParentOptions, isProtectedRole } from "./role-utils";

export type RoleAuthorityMode = "create" | "edit" | "authority" | "copy";

type AuthorityTab = "basic" | "permissions" | "menus" | "buttons" | "apis" | "data";

const TAB_ITEMS: Array<{ id: AuthorityTab; label: string }> = [
  { id: "basic", label: "基本信息" },
  { id: "permissions", label: "功能权限" },
  { id: "menus", label: "菜单权限" },
  { id: "buttons", label: "按钮权限" },
  { id: "apis", label: "API 权限" },
  { id: "data", label: "数据权限" },
];

const ACTION_LABELS: Record<DataAction, string> = {
  select: "查询",
  insert: "新增",
  update: "修改",
  delete: "删除",
};

const DEFAULT_SCOPES: Record<string, string> = {
  project: "member_set",
  agent: "self",
  member: "all",
};

interface ResourceRow {
  resourceKey: string;
  title: string;
  tableLabel: string;
  description: string;
  scope: string;
  actions: Record<DataAction, boolean>;
}

interface RoleAuthorityEditorProps {
  open: boolean;
  mode: RoleAuthorityMode;
  workspaceId: string;
  tree: RoleTreeVM[];
  role?: RoleTreeVM | null;
  parentId?: string;
  onClose: () => void;
  onSaved: () => void;
}

function apiRowKey(api: RegisteredAPI) {
  return `${api.method.toUpperCase()} ${api.path}`;
}

function normalizeRegisteredAPI(raw: RegisteredAPI & Record<string, unknown>): RegisteredAPI {
  return {
    id: String(raw.id ?? raw.ID ?? ""),
    path: String(raw.path ?? raw.Path ?? ""),
    method: String(raw.method ?? raw.Method ?? "GET").toUpperCase(),
    api_group: String(raw.api_group ?? raw.APIGroup ?? "default"),
    description: String(raw.description ?? raw.Description ?? ""),
  };
}

function buildDataRows(resources: DataResourceVM[], policies: RoleDataPolicyVM[]): ResourceRow[] {
  const policyMap = new Map<string, RoleDataPolicyVM[]>();
  for (const policy of policies) {
    const list = policyMap.get(policy.resource_key) ?? [];
    list.push(policy);
    policyMap.set(policy.resource_key, list);
  }
  return resources.map((resource) => {
    const existing = policyMap.get(resource.resource_key) ?? [];
    const scope =
      existing.find((p) => p.data_scope)?.data_scope ??
      DEFAULT_SCOPES[resource.resource_key] ??
      "self";
    const actions = Object.fromEntries(
      DATA_ACTIONS.map((action) => [action, existing.some((p) => p.action === action)]),
    ) as Record<DataAction, boolean>;
    return {
      resourceKey: resource.resource_key,
      title: resource.title || resource.resource_key,
      tableLabel: `${resource.schema_name}.${resource.table_name}`,
      description: resource.description,
      scope,
      actions,
    };
  });
}

function rowsToPolicies(rows: ResourceRow[]): RoleDataPolicyVM[] {
  const items: RoleDataPolicyVM[] = [];
  for (const row of rows) {
    for (const action of DATA_ACTIONS) {
      if (!row.actions[action]) continue;
      items.push({ resource_key: row.resourceKey, action, data_scope: row.scope });
    }
  }
  return items;
}

export function RoleAuthorityEditor({
  open,
  mode,
  workspaceId,
  tree,
  role,
  parentId,
  onClose,
  onSaved,
}: RoleAuthorityEditorProps) {
  const [tab, setTab] = useState<AuthorityTab>("basic");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const [name, setName] = useState("");
  const [selectedParentId, setSelectedParentId] = useState("");
  const [permissions, setPermissions] = useState<string[]>([]);
  const [menuTree, setMenuTree] = useState<MenuTreeVM[]>([]);
  const [menuQuery, setMenuQuery] = useState("");
  const [selectedMenuIds, setSelectedMenuIds] = useState<Set<string>>(new Set());
  const [buttonCatalog, setButtonCatalog] = useState<WorkspaceMenuButtonVM[]>([]);
  const [selectedButtonIds, setSelectedButtonIds] = useState<Set<string>>(new Set());
  const [buttonQuery, setButtonQuery] = useState("");
  const [apis, setApis] = useState<RegisteredAPI[]>([]);
  const [apiQuery, setApiQuery] = useState("");
  const [selectedApiKeys, setSelectedApiKeys] = useState<Set<string>>(new Set());
  const [dataRows, setDataRows] = useState<ResourceRow[]>([]);

  const visibleTabs = useMemo(() => {
    if (mode === "copy") return TAB_ITEMS.filter((t) => t.id === "basic");
    return TAB_ITEMS;
  }, [mode]);

  const title =
    mode === "create"
      ? parentId
        ? "新增子角色"
        : "新增角色"
      : mode === "edit"
        ? "编辑角色"
        : mode === "copy"
          ? "拷贝角色"
          : "设置权限";

  const loadAll = useCallback(async () => {
    if (!open) return;
    setLoading(true);
    setError(null);
    try {
      if (mode === "copy" && role) {
        setName(`${role.name}_copy`);
        setSelectedParentId(role.parent_id ?? "");
        return;
      }

      const [menus, buttons, apiList, resources] = await Promise.all([
        governance.listMenuTree(workspaceId),
        governance.listWorkspaceMenuButtons(workspaceId),
        governance.listWorkspaceApis(workspaceId),
        governance.listDataResources(workspaceId),
      ]);

      setMenuTree(menus);
      setButtonCatalog(buttons);
      setApis(
        apiList
          .map((item) => normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>))
          .filter((api) => api.path),
      );
      setDataRows(buildDataRows(resources, []));

      if (mode === "create") {
        setName("");
        setSelectedParentId(parentId ?? "");
        setPermissions([]);
        setSelectedMenuIds(new Set());
        setSelectedButtonIds(new Set());
        setSelectedApiKeys(new Set());
        return;
      }

      if (!role) return;
      setName(role.name);
      setSelectedParentId(role.parent_id ?? "");
      setPermissions([...role.permissions]);

      const [menuIds, buttonIds, apiPolicies, dataPolicies] = await Promise.all([
        governance.getRoleMenus(workspaceId, role.id),
        governance.getRoleMenuButtons(workspaceId, role.id),
        governance.listRoleAPIPolicies(workspaceId, role.id),
        governance.listRoleDataPolicies(workspaceId, role.id),
      ]);
      setSelectedMenuIds(new Set(menuIds));
      setSelectedButtonIds(new Set(buttonIds));
      setSelectedApiKeys(new Set(apiPolicies.map((p) => `${p.method.toUpperCase()} ${p.path}`)));
      setDataRows(buildDataRows(resources, dataPolicies));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [open, mode, role, parentId, workspaceId]);

  useEffect(() => {
    if (open) {
      setTab(mode === "authority" ? "menus" : "basic");
      void loadAll();
    }
  }, [open, loadAll, mode]);

  const parentOptions = collectRoleParentOptions(tree, mode === "edit" ? role?.id : undefined);
  const apiGroups = useMemo(() => {
    const groups = new Set<string>();
    for (const api of apis) groups.add(api.api_group || "default");
    return Array.from(groups).sort();
  }, [apis]);

  const filteredButtons = useMemo(() => {
    const q = buttonQuery.trim().toLowerCase();
    if (!q) return buttonCatalog;
    return buttonCatalog.filter(
      (item) =>
        item.name.toLowerCase().includes(q) ||
        item.code.toLowerCase().includes(q) ||
        item.menu_title.toLowerCase().includes(q) ||
        item.menu_path.toLowerCase().includes(q),
    );
  }, [buttonCatalog, buttonQuery]);

  const buttonsByMenu = useMemo(() => {
    const map = new Map<string, WorkspaceMenuButtonVM[]>();
    for (const item of filteredButtons) {
      const list = map.get(item.menu_id) ?? [];
      list.push(item);
      map.set(item.menu_id, list);
    }
    return map;
  }, [filteredButtons]);

  function toggleApi(api: RegisteredAPI) {
    const key = apiRowKey(api);
    setSelectedApiKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  function toggleButton(buttonId: string) {
    setSelectedButtonIds((prev) => {
      const next = new Set(prev);
      if (next.has(buttonId)) next.delete(buttonId);
      else next.add(buttonId);
      return next;
    });
  }

  function updateDataRow(resourceKey: string, patch: Partial<ResourceRow>) {
    setDataRows((prev) => prev.map((row) => (row.resourceKey === resourceKey ? { ...row, ...patch } : row)));
  }

  function toggleDataAction(resourceKey: string, action: DataAction) {
    setDataRows((prev) =>
      prev.map((row) =>
        row.resourceKey === resourceKey
          ? { ...row, actions: { ...row.actions, [action]: !row.actions[action] } }
          : row,
      ),
    );
  }

  async function saveAuthority(roleId: string) {
    const apiItems: RoleAPIPolicy[] = apis
      .filter((api) => selectedApiKeys.has(apiRowKey(api)))
      .map((api) => ({ path: api.path, method: api.method }));
    await Promise.all([
      governance.setRoleMenus(workspaceId, roleId, { menu_ids: Array.from(selectedMenuIds) }),
      governance.setRoleMenuButtons(workspaceId, roleId, { button_ids: Array.from(selectedButtonIds) }),
      governance.updateRoleAPIPolicies(workspaceId, roleId, apiItems),
      governance.updateRoleDataPolicies(workspaceId, roleId, rowsToPolicies(dataRows)),
    ]);
  }

  async function onSave() {
    setSaving(true);
    setError(null);
    try {
      if (mode === "copy") {
        if (!role) return;
        await governance.copyRole(workspaceId, role.id, name.trim());
        onSaved();
        onClose();
        return;
      }

      if (!name.trim()) {
        setError("请输入角色名称");
        setTab("basic");
        return;
      }

      if (mode === "create") {
        if (permissions.length === 0) {
          setError("至少选择一个功能权限");
          setTab("permissions");
          return;
        }
        const created = await governance.createRole(workspaceId, {
          name: name.trim(),
          permissions,
          parent_id: selectedParentId || undefined,
        });
        await saveAuthority(created.id);
      } else if (role) {
        await governance.updateRole(workspaceId, role.id, {
          name: name.trim(),
          parent_id: selectedParentId || null,
          permissions,
        });
        await saveAuthority(role.id);
      }

      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Drawer
      open={open}
      onClose={onClose}
      title={role && mode !== "create" ? `${title} · ${role.name}` : title}
      description="参考 GVA 角色管理：统一配置功能、菜单、按钮、API 与数据访问范围。"
      panelClassName="drawer-panel-wide"
      tabs={
        <>
          {visibleTabs.map((item) => (
            <button
              key={item.id}
              type="button"
              className={`drawer-tab ${tab === item.id ? "drawer-tab-active" : ""}`}
              onClick={() => setTab(item.id)}
            >
              {item.label}
            </button>
          ))}
        </>
      }
      footer={
        <div className="modal-footer-actions">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button type="button" className="btn" disabled={saving || loading} onClick={() => void onSave()}>
            {saving ? "保存中…" : mode === "create" ? "创建角色" : "保存"}
          </button>
        </div>
      }
    >
      <div className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {loading ? (
          <div className="empty-state">加载中…</div>
        ) : tab === "basic" ? (
          <div className="stack-md">
            <label className="create-field">
              <span className="text-sm font-medium">角色名称</span>
              <input
                className="input"
                placeholder="请输入角色名称"
                value={name}
                onChange={(e) => setName(e.target.value)}
                disabled={mode === "edit" && Boolean(role && isProtectedRole(role.name))}
                required
              />
            </label>
            {mode !== "copy" ? (
              <label className="create-field">
                <span className="text-sm font-medium">父级角色</span>
                <select
                  className="input"
                  value={selectedParentId}
                  onChange={(e) => setSelectedParentId(e.target.value)}
                >
                  <option value="">无（根角色）</option>
                  {parentOptions.map((option) => (
                    <option key={option.id} value={option.id}>
                      {"—".repeat(option.depth)}
                      {option.depth > 0 ? " " : ""}
                      {option.label}
                    </option>
                  ))}
                </select>
              </label>
            ) : (
              <p className="text-sm text-muted">
                将复制源角色的功能权限、菜单、按钮、API 与数据策略。
              </p>
            )}
          </div>
        ) : tab === "permissions" ? (
          <div className="grid gap-2 sm:grid-cols-2">
            {PERMISSION_OPTIONS.map((option) => (
              <label key={option.value} className="role-perm-card">
                <input
                  type="checkbox"
                  checked={permissions.includes(option.value)}
                  onChange={() =>
                    setPermissions((prev) =>
                      prev.includes(option.value)
                        ? prev.filter((p) => p !== option.value)
                        : [...prev, option.value],
                    )
                  }
                />
                <span>{option.label}</span>
              </label>
            ))}
          </div>
        ) : tab === "menus" ? (
          <div className="stack-sm">
            <input
              className="input"
              placeholder="搜索菜单名称或路径…"
              value={menuQuery}
              onChange={(e) => setMenuQuery(e.target.value)}
            />
            {menuTree.length === 0 ? (
              <div className="empty-state">暂无菜单，请先在菜单管理中初始化</div>
            ) : (
              <MenuAuthorityTree
                tree={menuTree}
                selected={selectedMenuIds}
                onChange={setSelectedMenuIds}
                query={menuQuery}
              />
            )}
          </div>
        ) : tab === "buttons" ? (
          <div className="stack-sm">
            <input
              className="input"
              placeholder="搜索按钮或所属菜单…"
              value={buttonQuery}
              onChange={(e) => setButtonQuery(e.target.value)}
            />
            {filteredButtons.length === 0 ? (
              <div className="empty-state">暂无按钮定义，可在菜单管理中为页面配置按钮</div>
            ) : (
              <div className="role-button-groups max-h-[60vh] overflow-y-auto stack-md">
                {Array.from(buttonsByMenu.entries()).map(([menuId, items]) => (
                  <section key={menuId} className="role-button-group">
                    <header className="role-button-group-head">
                      <h4>{items[0]?.menu_title}</h4>
                      <span className="font-mono text-xs text-muted">{items[0]?.menu_path}</span>
                    </header>
                    <div className="role-button-grid">
                      {items.map((btn) => (
                        <label key={btn.button_id} className="role-perm-card">
                          <input
                            type="checkbox"
                            checked={selectedButtonIds.has(btn.button_id)}
                            onChange={() => toggleButton(btn.button_id)}
                          />
                          <span>
                            <span className="font-medium">{btn.name}</span>
                            <span className="block text-xs text-muted font-mono">{btn.code}</span>
                          </span>
                        </label>
                      ))}
                    </div>
                  </section>
                ))}
              </div>
            )}
          </div>
        ) : tab === "apis" ? (
          <div className="stack-sm">
            <input
              className="input"
              placeholder="搜索 API 路径或描述…"
              value={apiQuery}
              onChange={(e) => setApiQuery(e.target.value)}
            />
            {apis.length === 0 ? (
              <div className="empty-state">工作区暂无已注册 API，请先在 API 管理中同步</div>
            ) : (
              <div className="stack-md max-h-[60vh] overflow-y-auto">
                {apiGroups.map((group) => {
                  const groupApis = apis.filter((api) => {
                    if ((api.api_group || "default") !== group) return false;
                    const q = apiQuery.trim().toLowerCase();
                    if (!q) return true;
                    return (
                      api.path.toLowerCase().includes(q) ||
                      api.method.toLowerCase().includes(q) ||
                      (api.description ?? "").toLowerCase().includes(q)
                    );
                  });
                  if (groupApis.length === 0) return null;
                  return (
                    <section key={group}>
                      <p className="text-xs font-semibold text-muted mb-2">{group}</p>
                      <div className="grid gap-2">
                        {groupApis.map((api) => {
                          const key = apiRowKey(api);
                          return (
                            <label key={api.id || key} className="role-perm-card">
                              <input
                                type="checkbox"
                                checked={selectedApiKeys.has(key)}
                                onChange={() => toggleApi(api)}
                              />
                              <span>
                                <span className="badge badge-neutral mr-2">{api.method}</span>
                                <span className="font-mono text-xs">{api.path}</span>
                                {api.description ? (
                                  <span className="block text-muted text-xs mt-1">{api.description}</span>
                                ) : null}
                              </span>
                            </label>
                          );
                        })}
                      </div>
                    </section>
                  );
                })}
              </div>
            )}
          </div>
        ) : dataRows.length === 0 ? (
          <div className="empty-state">暂无已注册的数据资源</div>
        ) : (
          <div className="table-wrap max-h-[60vh] overflow-y-auto">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  <th>数据资源</th>
                  {DATA_ACTIONS.map((action) => (
                    <th key={action} className="text-center w-16">
                      {ACTION_LABELS[action]}
                    </th>
                  ))}
                  <th className="min-w-[8rem]">数据范围</th>
                </tr>
              </thead>
              <tbody>
                {dataRows.map((row) => (
                  <tr key={row.resourceKey}>
                    <td>
                      <div className="font-medium">{row.title}</div>
                      <div className="font-mono text-xs text-muted">{row.tableLabel}</div>
                    </td>
                    {DATA_ACTIONS.map((action) => (
                      <td key={action} className="text-center">
                        <input
                          type="checkbox"
                          checked={row.actions[action]}
                          onChange={() => toggleDataAction(row.resourceKey, action)}
                        />
                      </td>
                    ))}
                    <td>
                      <select
                        className="input !py-1 !text-xs w-full"
                        value={row.scope}
                        onChange={(e) => updateDataRow(row.resourceKey, { scope: e.target.value })}
                      >
                        {DATA_SCOPES.map((option) => (
                          <option key={option.value} value={option.value}>
                            {option.label}
                          </option>
                        ))}
                      </select>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </Drawer>
  );
}
