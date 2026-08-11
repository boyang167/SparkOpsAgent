"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError, PERMISSION_OPTIONS } from "@sparkops/contracts";
import {
  DATA_ACTIONS,
  DATA_SCOPES,
  type DataAction,
  type DataResourceVM,
  type RegisteredAPI,
  type RoleAPIPolicy,
  type RoleDataPolicyVM,
  type RoleTreeVM,
} from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Drawer } from "@/components/ui/Drawer";
import { flattenMenuTree } from "../menus/menu-utils";

type AuthorityTab = "permissions" | "menus" | "apis" | "data";

const TAB_ITEMS: Array<{ id: AuthorityTab; label: string }> = [
  { id: "permissions", label: "功能权限" },
  { id: "menus", label: "菜单权限" },
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
      items.push({
        resource_key: row.resourceKey,
        action,
        data_scope: row.scope,
      });
    }
  }
  return items;
}

interface RoleAuthorityDrawerProps {
  workspaceId: string;
  role: RoleTreeVM | null;
  onClose: () => void;
  onSaved: () => void;
}

export function RoleAuthorityDrawer({ workspaceId, role, onClose, onSaved }: RoleAuthorityDrawerProps) {
  const [tab, setTab] = useState<AuthorityTab>("permissions");
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [success, setSuccess] = useState<string | null>(null);

  const [permissions, setPermissions] = useState<string[]>([]);
  const [menuRows, setMenuRows] = useState<Array<{ id: string; title: string; path: string; depth: number }>>(
    [],
  );
  const [selectedMenuIds, setSelectedMenuIds] = useState<Set<string>>(new Set());
  const [apis, setApis] = useState<RegisteredAPI[]>([]);
  const [selectedApiKeys, setSelectedApiKeys] = useState<Set<string>>(new Set());
  const [dataRows, setDataRows] = useState<ResourceRow[]>([]);

  const loadTab = useCallback(async () => {
    if (!role) return;
    setLoading(true);
    setError(null);
    try {
      if (tab === "permissions") {
        setPermissions([...role.permissions]);
      } else if (tab === "menus") {
        const [menuTree, menuIds] = await Promise.all([
          governance.listMenuTree(workspaceId),
          governance.getRoleMenus(workspaceId, role.id),
        ]);
        setMenuRows(flattenMenuTree(menuTree));
        setSelectedMenuIds(new Set(menuIds));
      } else if (tab === "apis") {
        const [apiList, policies] = await Promise.all([
          governance.listWorkspaceApis(workspaceId),
          governance.listRoleAPIPolicies(workspaceId, role.id),
        ]);
        const normalized = apiList
          .map((item) => normalizeRegisteredAPI(item as RegisteredAPI & Record<string, unknown>))
          .filter((api) => api.path);
        setApis(normalized);
        setSelectedApiKeys(new Set(policies.map((p) => `${p.method.toUpperCase()} ${p.path}`)));
      } else {
        const [resources, policies] = await Promise.all([
          governance.listDataResources(workspaceId),
          governance.listRoleDataPolicies(workspaceId, role.id),
        ]);
        setDataRows(buildDataRows(resources, policies));
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, role, tab]);

  useEffect(() => {
    if (role) {
      setTab("permissions");
      setSuccess(null);
    }
  }, [role?.id]);

  useEffect(() => {
    void loadTab();
  }, [loadTab]);

  const apiGroups = useMemo(() => {
    const groups = new Set<string>();
    for (const api of apis) groups.add(api.api_group || "default");
    return Array.from(groups).sort();
  }, [apis]);

  function flashSuccess(message: string) {
    setSuccess(message);
    window.setTimeout(() => setSuccess(null), 2400);
  }

  function toggleMenu(menuId: string) {
    setSelectedMenuIds((prev) => {
      const next = new Set(prev);
      if (next.has(menuId)) next.delete(menuId);
      else next.add(menuId);
      return next;
    });
  }

  function toggleApi(api: RegisteredAPI) {
    const key = apiRowKey(api);
    setSelectedApiKeys((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
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

  async function onSave() {
    if (!role) return;
    setSaving(true);
    setError(null);
    try {
      if (tab === "permissions") {
        if (permissions.length === 0) {
          setError("至少选择一个功能权限");
          return;
        }
        await governance.updateRole(workspaceId, role.id, { permissions });
      } else if (tab === "menus") {
        await governance.setRoleMenus(workspaceId, role.id, {
          menu_ids: Array.from(selectedMenuIds),
        });
      } else if (tab === "apis") {
        const items: RoleAPIPolicy[] = apis
          .filter((api) => selectedApiKeys.has(apiRowKey(api)))
          .map((api) => ({ path: api.path, method: api.method }));
        await governance.updateRoleAPIPolicies(workspaceId, role.id, items);
      } else {
        await governance.updateRoleDataPolicies(workspaceId, role.id, rowsToPolicies(dataRows));
      }
      flashSuccess("保存成功");
      onSaved();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Drawer
      open={Boolean(role)}
      onClose={onClose}
      title={role ? `设置权限 · ${role.name}` : "设置权限"}
      description="按 Tab 分别配置功能、菜单、API 与数据权限，每个 Tab 独立保存。"
      tabs={
        <>
          {TAB_ITEMS.map((item) => (
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
            关闭
          </button>
          <button type="button" className="btn" disabled={saving || loading} onClick={() => void onSave()}>
            {saving ? "保存中…" : "确定"}
          </button>
        </div>
      }
    >
      <div className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {success ? <InlineAlert tone="success">{success}</InlineAlert> : null}
        {loading ? (
          <div className="empty-state">加载中…</div>
        ) : tab === "permissions" ? (
          <div className="grid gap-2 sm:grid-cols-2">
            {PERMISSION_OPTIONS.map((option) => (
              <label key={option.value} className="flex items-center gap-2 text-sm border rounded p-2">
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
                {option.label}
              </label>
            ))}
          </div>
        ) : tab === "menus" ? (
          menuRows.length === 0 ? (
            <div className="empty-state">暂无菜单，请先在菜单管理中初始化</div>
          ) : (
            <div className="max-h-[60vh] overflow-y-auto grid gap-1">
              {menuRows.map((menu) => (
                <label
                  key={menu.id}
                  className="flex items-center gap-2 text-sm border rounded p-2"
                  style={{ paddingLeft: `${menu.depth * 1 + 0.5}rem` }}
                >
                  <input
                    type="checkbox"
                    checked={selectedMenuIds.has(menu.id)}
                    onChange={() => toggleMenu(menu.id)}
                  />
                  <span>
                    <span className="font-medium">{menu.title}</span>
                    <span className="block font-mono text-xs text-muted">{menu.path}</span>
                  </span>
                </label>
              ))}
            </div>
          )
        ) : tab === "apis" ? (
          apis.length === 0 ? (
            <div className="empty-state">工作区暂无已注册 API，请先在 API 管理中同步</div>
          ) : (
            <div className="stack-md">
              {apiGroups.map((group) => (
                <div key={group}>
                  <p className="text-xs font-semibold text-muted mb-2">{group}</p>
                  <div className="grid gap-2">
                    {apis
                      .filter((api) => (api.api_group || "default") === group)
                      .map((api) => {
                        const key = apiRowKey(api);
                        return (
                          <label key={api.id || key} className="flex items-start gap-2 text-sm border rounded p-2">
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
                </div>
              ))}
            </div>
          )
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
