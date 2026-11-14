"use client";

import { useCallback, useEffect, useState } from "react";
import type { CreateStorageBindingCommand, StorageBindingVM, StoragePoolVM } from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import { createStorageBinding, listStorageBindings, listStoragePools } from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { canMutateStorage } from "./storage-admin-permissions";

const SCOPE_TYPES = ["ORGANIZATION", "WORKSPACE", "PROJECT", "USER"] as const;

export type StorageBindingPanelProps = {
  workspaceId: string;
  permissions: string[];
  bindings?: StorageBindingVM[];
};

export function StorageBindingPanel({
  workspaceId,
  permissions,
  bindings: initialBindings,
}: StorageBindingPanelProps) {
  const canAdmin = canMutateStorage(permissions);
  const [bindings, setBindings] = useState<StorageBindingVM[]>(initialBindings ?? []);
  const [pools, setPools] = useState<StoragePoolVM[]>([]);
  const [loading, setLoading] = useState(!initialBindings);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [form, setForm] = useState<CreateStorageBindingCommand>({
    scope_type: "WORKSPACE",
    scope_id: workspaceId,
    pool_id: "",
    priority: 0,
  });

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [bindingPage, poolPage] = await Promise.all([
        listStorageBindings(workspaceId),
        listStoragePools(workspaceId),
      ]);
      setBindings(bindingPage.items);
      setPools(poolPage.items);
      if (poolPage.items[0]) {
        setForm((current) =>
          current.pool_id ? current : { ...current, pool_id: poolPage.items[0].id },
        );
      }
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载存储绑定失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    if (initialBindings === undefined) {
      void load();
    }
  }, [initialBindings, load]);

  async function submit() {
    if (!canAdmin || !form.pool_id || !form.scope_id) return;
    setSubmitting(true);
    setError(null);
    try {
      await createStorageBinding(workspaceId, form);
      setOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建绑定失败");
    } finally {
      setSubmitting(false);
    }
  }

  const poolNameById = new Map(pools.map((pool) => [pool.id, pool.name]));

  return (
    <section className="stack-lg">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-semibold">存储作用域绑定</h2>
          <p className="text-sm text-muted">将存储池绑定到 Organization / Workspace / Project / User。</p>
        </div>
        {canAdmin ? (
          <button
            type="button"
            className={open ? "btn btn-secondary" : "btn btn-primary"}
            onClick={() => setOpen(true)}
          >
            新增绑定
          </button>
        ) : null}
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : bindings.length === 0 ? (
        <div className="empty-state">暂无绑定</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>作用域</th>
                <th>Scope ID</th>
                <th>存储池</th>
                <th>优先级</th>
              </tr>
            </thead>
            <tbody>
              {bindings.map((binding) => (
                <tr key={binding.id}>
                  <td>{binding.scope_type}</td>
                  <td className="font-mono text-xs">{binding.scope_id}</td>
                  <td>{poolNameById.get(binding.pool_id) ?? binding.pool_id.slice(0, 8)}</td>
                  <td>{binding.priority}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={open} title="新增存储绑定" onClose={() => setOpen(false)}>
        <div className="stack-md">
          <label className="stack-xs">
            <span>作用域类型</span>
            <select
              className="input"
              value={form.scope_type}
              onChange={(e) =>
                setForm((f) => ({
                  ...f,
                  scope_type: e.target.value as CreateStorageBindingCommand["scope_type"],
                }))
              }
            >
              {SCOPE_TYPES.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span>Scope ID</span>
            <input
              className="input font-mono text-xs"
              value={form.scope_id}
              onChange={(e) => setForm((f) => ({ ...f, scope_id: e.target.value }))}
            />
          </label>
          <label className="stack-xs">
            <span>存储池</span>
            <select
              className="input"
              value={form.pool_id}
              onChange={(e) => setForm((f) => ({ ...f, pool_id: e.target.value }))}
            >
              <option value="">选择存储池</option>
              {pools.map((pool) => (
                <option key={pool.id} value={pool.id}>
                  {pool.name}
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span>优先级</span>
            <input
              type="number"
              className="input"
              value={form.priority ?? 0}
              onChange={(e) => setForm((f) => ({ ...f, priority: Number(e.target.value) }))}
            />
          </label>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn btn-ghost" disabled={submitting} onClick={() => setOpen(false)}>
              取消
            </button>
            <button type="button" className="btn btn-primary" disabled={submitting} onClick={() => void submit()}>
              保存
            </button>
          </div>
        </div>
      </Modal>
    </section>
  );
}
