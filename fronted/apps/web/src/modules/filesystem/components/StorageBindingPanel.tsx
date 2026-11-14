"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type { CreateStorageBindingCommand, StorageBindingVM, StoragePoolVM } from "@sparkops/contracts/filesystem";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { BffError } from "@sparkops/contracts";
import { createStorageBinding, listStorageBindings, listStoragePools } from "@sparkops/web-sdk/filesystem";
import { tenant, workspace } from "@sparkops/web-sdk";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { canMutateStorage } from "./storage-admin-permissions";
import {
  defaultScopeIdForType,
  formatBindingScopeRow,
  scopeTargetOptions,
  STORAGE_BINDING_SCOPE_LABELS,
  STORAGE_BINDING_SCOPE_TYPES,
  validateStorageBindingForm,
} from "./storage-binding-panel-view";

export type StorageBindingPanelProps = {
  workspaceId: string;
  permissions: string[];
  bindings?: StorageBindingVM[];
};

function initialBindingForm(workspaceId: string): CreateStorageBindingCommand {
  return {
    scope_type: "WORKSPACE",
    scope_id: workspaceId,
    pool_id: "",
    priority: 0,
  };
}

export function StorageBindingPanel({
  workspaceId,
  permissions,
  bindings: initialBindings,
}: StorageBindingPanelProps) {
  const canAdmin = canMutateStorage(permissions);
  const [bindings, setBindings] = useState<StorageBindingVM[]>(initialBindings ?? []);
  const [pools, setPools] = useState<StoragePoolVM[]>([]);
  const [organizations, setOrganizations] = useState<OrganizationVM[]>([]);
  const [workspaces, setWorkspaces] = useState<WorkspaceVM[]>([]);
  const [projects, setProjects] = useState<ProjectVM[]>([]);
  const [loading, setLoading] = useState(!initialBindings);
  const [error, setError] = useState<string | null>(null);
  const [open, setOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);
  const [form, setForm] = useState<CreateStorageBindingCommand>(() => initialBindingForm(workspaceId));

  const catalog = useMemo(
    () => ({ organizations, workspaces, projects }),
    [organizations, workspaces, projects],
  );

  const scopeOptions = useMemo(
    () => scopeTargetOptions(form.scope_type, catalog),
    [form.scope_type, catalog],
  );

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [bindingPage, poolPage, orgItems, workspaceItems, projectItems] = await Promise.all([
        listStorageBindings(workspaceId),
        listStoragePools(workspaceId),
        tenant.listOrganizations(),
        tenant.listWorkspaces(),
        workspace.listProjects(workspaceId),
      ]);
      setBindings(bindingPage.items);
      setPools(poolPage.items);
      setOrganizations(orgItems);
      setWorkspaces(workspaceItems);
      setProjects(projectItems);
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

  useEffect(() => {
    setForm((current) =>
      current.scope_type === "WORKSPACE" && current.scope_id !== workspaceId
        ? { ...current, scope_id: workspaceId }
        : current,
    );
  }, [workspaceId]);

  function openCreateModal() {
    setFormError(null);
    setForm({
      ...initialBindingForm(workspaceId),
      pool_id: pools[0]?.id ?? "",
    });
    setOpen(true);
  }

  async function submit() {
    if (!canAdmin) return;
    const validationError = validateStorageBindingForm(form);
    if (validationError) {
      setFormError(validationError);
      return;
    }
    setFormError(null);
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
          <p className="text-sm text-muted">将存储池绑定到租户、工作区、项目或用户；按名称选择，无需手填 UUID。</p>
        </div>
        {canAdmin ? (
          <button
            type="button"
            className={open ? "btn btn-secondary" : "btn btn-primary"}
            onClick={openCreateModal}
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
                <th>绑定目标</th>
                <th>存储池</th>
                <th>优先级</th>
              </tr>
            </thead>
            <tbody>
              {bindings.map((binding) => {
                const row = formatBindingScopeRow(binding, catalog);
                return (
                  <tr key={binding.id}>
                    <td>{row.scopeLabel}</td>
                    <td>{row.targetLabel}</td>
                    <td>{poolNameById.get(binding.pool_id) ?? binding.pool_id.slice(0, 8)}</td>
                    <td>{binding.priority}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={open} title="新增存储绑定" onClose={() => setOpen(false)}>
        <div className="stack-md">
          {formError ? <InlineAlert tone="error">{formError}</InlineAlert> : null}
          <label className="stack-xs">
            <span>作用域类型</span>
            <select
              className="input"
              value={form.scope_type}
              onChange={(e) => {
                const scopeType = e.target.value as CreateStorageBindingCommand["scope_type"];
                setFormError(null);
                setForm((current) => ({
                  ...current,
                  scope_type: scopeType,
                  scope_id: defaultScopeIdForType(scopeType, workspaceId, catalog),
                }));
              }}
            >
              {STORAGE_BINDING_SCOPE_TYPES.map((type) => (
                <option key={type} value={type}>
                  {STORAGE_BINDING_SCOPE_LABELS[type]}
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span>{form.scope_type === "USER" ? "用户 ID" : "绑定目标"}</span>
            {form.scope_type === "USER" ? (
              <>
                <input
                  className="input font-mono text-xs"
                  placeholder="用户 UUID"
                  value={form.scope_id}
                  onChange={(e) => {
                    setFormError(null);
                    setForm((f) => ({ ...f, scope_id: e.target.value }));
                  }}
                />
                <span className="text-xs text-muted">用户级绑定暂仅支持按 ID 指定。</span>
              </>
            ) : scopeOptions.length === 0 ? (
              <span className="text-sm text-muted">
                {form.scope_type === "PROJECT" ? "当前工作区下暂无项目，请先创建项目。" : "暂无可选目标。"}
              </span>
            ) : (
              <select
                className="input"
                value={form.scope_id}
                onChange={(e) => {
                  setFormError(null);
                  setForm((f) => ({ ...f, scope_id: e.target.value }));
                }}
              >
                {scopeOptions.map((option) => (
                  <option key={option.id} value={option.id}>
                    {option.detail ? `${option.label}（${option.detail}）` : option.label}
                  </option>
                ))}
              </select>
            )}
          </label>
          <label className="stack-xs">
            <span>存储池</span>
            <select
              className="input"
              value={form.pool_id}
              onChange={(e) => {
                setFormError(null);
                setForm((f) => ({ ...f, pool_id: e.target.value }));
              }}
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
            <span className="text-xs text-muted">同一作用域多条绑定时，数字越大越优先。</span>
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