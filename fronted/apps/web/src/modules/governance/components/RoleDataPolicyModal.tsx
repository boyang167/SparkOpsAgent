"use client";

import { useCallback, useEffect, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import {
  DATA_ACTIONS,
  DATA_SCOPES,
  type DataAction,
  type DataResourceVM,
  type RoleDataPolicyVM,
  type RoleTreeVM,
} from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

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

function buildRows(resources: DataResourceVM[], policies: RoleDataPolicyVM[]): ResourceRow[] {
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
      DATA_ACTIONS.map((action) => [
        action,
        existing.some((p) => p.action === action),
      ]),
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

interface RoleDataPolicyModalProps {
  workspaceId: string;
  role: RoleTreeVM | null;
  onClose: () => void;
  onSaved: () => void;
}

export function RoleDataPolicyModal({ workspaceId, role, onClose, onSaved }: RoleDataPolicyModalProps) {
  const [rows, setRows] = useState<ResourceRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!role) return;
    setLoading(true);
    setError(null);
    try {
      const [resources, policies] = await Promise.all([
        governance.listDataResources(workspaceId),
        governance.listRoleDataPolicies(workspaceId, role.id),
      ]);
      setRows(buildRows(resources, policies));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, role]);

  useEffect(() => {
    void load();
  }, [load]);

  function updateRow(resourceKey: string, patch: Partial<ResourceRow>) {
    setRows((prev) => prev.map((row) => (row.resourceKey === resourceKey ? { ...row, ...patch } : row)));
  }

  function toggleAction(resourceKey: string, action: DataAction) {
    setRows((prev) =>
      prev.map((row) =>
        row.resourceKey === resourceKey
          ? { ...row, actions: { ...row.actions, [action]: !row.actions[action] } }
          : row,
      ),
    );
  }

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    if (!role) return;
    setSaving(true);
    setError(null);
    try {
      await governance.updateRoleDataPolicies(workspaceId, role.id, rowsToPolicies(rows));
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={Boolean(role)} onClose={onClose} title={role ? `数据权限 · ${role.name}` : ""}>
      <form onSubmit={onSave} className="stack-md">
        <p className="text-sm text-muted">
          为角色配置各数据资源的访问范围。数据范围对该资源下已勾选的操作生效。
        </p>
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {loading ? (
          <div className="empty-state">加载中…</div>
        ) : rows.length === 0 ? (
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
                {rows.map((row) => (
                  <tr key={row.resourceKey}>
                    <td>
                      <div className="font-medium">{row.title}</div>
                      <div className="font-mono text-xs text-muted">{row.tableLabel}</div>
                      {row.description ? (
                        <div className="text-xs text-muted mt-1">{row.description}</div>
                      ) : null}
                    </td>
                    {DATA_ACTIONS.map((action) => (
                      <td key={action} className="text-center">
                        <input
                          type="checkbox"
                          checked={row.actions[action]}
                          onChange={() => toggleAction(row.resourceKey, action)}
                        />
                      </td>
                    ))}
                    <td>
                      <select
                        className="input !py-1 !text-xs w-full"
                        value={row.scope}
                        onChange={(e) => updateRow(row.resourceKey, { scope: e.target.value })}
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
        <div className="flex justify-end gap-2">
          <button type="button" className="btn-secondary" onClick={onClose}>
            取消
          </button>
          <button type="submit" className="btn" disabled={saving || loading}>
            {saving ? "保存中…" : "保存"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
