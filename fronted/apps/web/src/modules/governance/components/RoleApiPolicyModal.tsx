"use client";

import { useCallback, useEffect, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { RegisteredAPI, RoleAPIPolicy, RoleTreeVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

function normalizeRegisteredAPI(raw: RegisteredAPI & Record<string, unknown>): RegisteredAPI {
  return {
    id: String(raw.id ?? raw.ID ?? ""),
    path: String(raw.path ?? raw.Path ?? ""),
    method: String(raw.method ?? raw.Method ?? "GET").toUpperCase(),
    api_group: String(raw.api_group ?? raw.APIGroup ?? "default"),
    description: String(raw.description ?? raw.Description ?? ""),
  };
}

function apiRowKey(api: RegisteredAPI) {
  return `${api.method} ${api.path}`;
}

interface RoleApiPolicyModalProps {
  workspaceId: string;
  role: RoleTreeVM | null;
  onClose: () => void;
  onSaved: () => void;
}

export function RoleApiPolicyModal({ workspaceId, role, onClose, onSaved }: RoleApiPolicyModalProps) {
  const [apis, setApis] = useState<RegisteredAPI[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    if (!role) return;
    setLoading(true);
    setError(null);
    try {
      const [apiList, policies] = await Promise.all([
        governance.listWorkspaceApis(workspaceId),
        governance.listRoleAPIPolicies(workspaceId, role.id),
      ]);
      setApis(apiList.filter((api) => api.path));
      const keys = new Set(policies.map((p) => `${p.method.toUpperCase()} ${p.path}`));
      setSelected(keys);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId, role]);

  useEffect(() => {
    void load();
  }, [load]);

  function toggle(api: RegisteredAPI) {
    const key = apiRowKey(api);
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(key)) next.delete(key);
      else next.add(key);
      return next;
    });
  }

  async function onSave(e: React.FormEvent) {
    e.preventDefault();
    if (!role) return;
    setSaving(true);
    setError(null);
    try {
      const items: RoleAPIPolicy[] = apis
        .filter((api) => selected.has(apiRowKey(api)))
        .map((api) => ({ path: api.path, method: api.method }));
      await governance.updateRoleAPIPolicies(workspaceId, role.id, items);
      onSaved();
      onClose();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "保存失败");
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={Boolean(role)} onClose={onClose} title={role ? `绑定 API · ${role.name}` : ""}>
      <form onSubmit={onSave} className="stack-md">
        {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
        {loading ? (
          <div className="empty-state">加载中…</div>
        ) : apis.length === 0 ? (
          <div className="empty-state">工作区暂无已注册 API，请先在 API 管理中同步</div>
        ) : (
          <div className="max-h-80 overflow-y-auto grid gap-2">
            {apis.map((api) => {
              const key = apiRowKey(api);
              return (
                <label key={api.id || key} className="flex items-start gap-2 text-sm border rounded p-2">
                  <input type="checkbox" checked={selected.has(key)} onChange={() => toggle(api)} />
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
