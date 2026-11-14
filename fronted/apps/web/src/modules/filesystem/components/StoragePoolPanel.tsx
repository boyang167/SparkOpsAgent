"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  CreateStoragePoolCommand,
  RegisterStorageProviderCommand,
  StoragePoolVM,
  StorageProviderVM,
  UpdateStorageProviderCommand,
} from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  createStoragePool,
  deleteStoragePool,
  deleteStorageProvider,
  listStoragePools,
  listStorageProviders,
  registerStorageProvider,
  updateStoragePool,
  updateStorageProvider,
} from "@sparkops/web-sdk/filesystem";
import { StoragePoolMembersEditor } from "./StoragePoolMembersEditor";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  canDeleteProvider,
  canShowStoragePoolMutations,
  createDefaultStoragePoolMembers,
  membersFromPool,
  normalizeCreateStoragePoolCommand,
  normalizeRegisterProviderCommand,
  normalizeUpdateProviderCommand,
  registerProviderSecretPlaceholder,
  STORAGE_POOL_CREATE_LABEL,
  STORAGE_PROVIDER_REGISTER_LABEL,
  type StoragePoolDraftMember,
  validateCreateStoragePoolForm,
  validateRegisterProviderForm,
  validateUpdateProviderForm,
} from "./storage-pool-panel-view";

export type StoragePoolPanelProps = {
  workspaceId: string;
  permissions: string[];
  providers?: StorageProviderVM[];
  pools?: StoragePoolVM[];
  onReload?: () => void;
};

const PROVIDER_TYPES = ["LOCAL", "NFS", "S3"] as const;
const PROVIDER_STATUSES = ["ACTIVE", "DISABLED"] as const;

type DeleteTarget = {
  kind: "provider" | "pool";
  id: string;
  name: string;
};

export function StoragePoolPanel({
  workspaceId,
  permissions,
  providers: initialProviders,
  pools: initialPools,
  onReload,
}: StoragePoolPanelProps) {
  const canAdmin = canShowStoragePoolMutations(permissions);
  const [providers, setProviders] = useState<StorageProviderVM[]>(initialProviders ?? []);
  const [pools, setPools] = useState<StoragePoolVM[]>(initialPools ?? []);
  const [loading, setLoading] = useState(!initialProviders);
  const [error, setError] = useState<string | null>(null);
  const [providerOpen, setProviderOpen] = useState(false);
  const [poolOpen, setPoolOpen] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [providerFormError, setProviderFormError] = useState<string | null>(null);
  const [poolFormError, setPoolFormError] = useState<string | null>(null);
  const [providerForm, setProviderForm] = useState<RegisterStorageProviderCommand>({
    name: "",
    provider_type: "S3",
    secret_ref: "",
    endpoint: "",
  });
  const [poolName, setPoolName] = useState("");
  const [poolMembers, setPoolMembers] = useState<StoragePoolDraftMember[]>([]);
  const [poolEditOpen, setPoolEditOpen] = useState(false);
  const [editingPool, setEditingPool] = useState<StoragePoolVM | null>(null);
  const [poolEditName, setPoolEditName] = useState("");
  const [poolEditStatus, setPoolEditStatus] = useState<StoragePoolVM["status"]>("ACTIVE");
  const [poolEditMembers, setPoolEditMembers] = useState<StoragePoolDraftMember[]>([]);
  const [providerEditOpen, setProviderEditOpen] = useState(false);
  const [editingProvider, setEditingProvider] = useState<StorageProviderVM | null>(null);
  const [providerEditForm, setProviderEditForm] = useState<UpdateStorageProviderCommand>({
    name: "",
    endpoint: "",
    secret_ref: "",
    status: "ACTIVE",
  });
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const [providerPage, poolPage] = await Promise.all([
        listStorageProviders(workspaceId),
        listStoragePools(workspaceId),
      ]);
      setProviders(providerPage.items);
      setPools(poolPage.items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载存储配置失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    if (initialProviders === undefined) {
      void load();
    }
  }, [initialProviders, load]);

  const providerNameById = useMemo(() => {
    const map = new Map<string, string>();
    for (const provider of providers) {
      map.set(provider.id, provider.name);
    }
    return map;
  }, [providers]);

  const activeProviders = useMemo(
    () => providers.filter((provider) => provider.status === "ACTIVE"),
    [providers],
  );

  async function submitProvider() {
    if (!canAdmin) return;
    const validationError = validateRegisterProviderForm(providerForm);
    if (validationError) {
      setProviderFormError(validationError);
      return;
    }
    setProviderFormError(null);
    setSubmitting(true);
    setError(null);
    try {
      await registerStorageProvider(workspaceId, normalizeRegisterProviderCommand(providerForm));
      setProviderOpen(false);
      setProviderForm({ name: "", provider_type: "S3", secret_ref: "", endpoint: "" });
      await load();
      onReload?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "注册 Provider 失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function submitPool() {
    if (!canAdmin) return;
    const validationError = validateCreateStoragePoolForm(poolName, poolMembers, providers);
    if (validationError) {
      setPoolFormError(validationError);
      return;
    }
    setPoolFormError(null);
    setSubmitting(true);
    setError(null);
    try {
      const input: CreateStoragePoolCommand = normalizeCreateStoragePoolCommand(
        poolName,
        poolMembers,
      );
      await createStoragePool(workspaceId, input);
      setPoolOpen(false);
      setPoolName("");
      setPoolMembers([]);
      await load();
      onReload?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建存储池失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function submitProviderUpdate() {
    if (!canAdmin || !editingProvider) return;
    const validationError = validateUpdateProviderForm(providerEditForm);
    if (validationError) {
      setProviderFormError(validationError);
      return;
    }
    setProviderFormError(null);
    setSubmitting(true);
    setError(null);
    try {
      await updateStorageProvider(
        workspaceId,
        editingProvider.id,
        normalizeUpdateProviderCommand(providerEditForm),
      );
      setProviderEditOpen(false);
      setEditingProvider(null);
      await load();
      onReload?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新 Provider 失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function submitPoolUpdate() {
    if (!canAdmin || !editingPool) return;
    const validationError = validateCreateStoragePoolForm(poolEditName, poolEditMembers, providers);
    if (validationError) {
      setPoolFormError(validationError);
      return;
    }
    setPoolFormError(null);
    setSubmitting(true);
    setError(null);
    try {
      await updateStoragePool(workspaceId, editingPool.id, {
        name: poolEditName.trim(),
        status: poolEditStatus,
        members: normalizeCreateStoragePoolCommand(poolEditName, poolEditMembers).members,
      });
      setPoolEditOpen(false);
      setEditingPool(null);
      await load();
      onReload?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新存储池失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function confirmDelete() {
    if (!canAdmin || !deleteTarget) return;
    setSubmitting(true);
    setError(null);
    try {
      if (deleteTarget.kind === "provider") {
        await deleteStorageProvider(workspaceId, deleteTarget.id);
      } else {
        await deleteStoragePool(workspaceId, deleteTarget.id);
      }
      setDeleteTarget(null);
      await load();
      onReload?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "删除失败");
    } finally {
      setSubmitting(false);
    }
  }

  const dialogOpen =
    providerOpen || poolOpen || providerEditOpen || poolEditOpen || deleteTarget !== null;

  return (
    <section className="stack-lg">
      <PanelHeader
        title="存储 Provider 与存储池"
        description="凭证仅通过 Secret 引用提交，界面不回显任何密钥内容。"
        canAdmin={canAdmin}
        dialogOpen={dialogOpen}
        onAddProvider={() => {
          setProviderFormError(null);
          setProviderOpen(true);
        }}
        onAddPool={() => {
          setPoolFormError(null);
          setPoolName("");
          setPoolMembers(createDefaultStoragePoolMembers(providers));
          setPoolOpen(true);
        }}
      />

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : (
        <>
          <PanelSubhead title="Provider" />
          {providers.length === 0 ? (
            <div className="empty-state">尚未注册 Provider</div>
          ) : (
            <div className="table-wrap">
              <table className="data-table text-sm">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>类型</th>
                    <th>Endpoint</th>
                    <th>凭证</th>
                    <th>状态</th>
                    {canAdmin ? <th>操作</th> : null}
                  </tr>
                </thead>
                <tbody>
                  {providers.map((provider) => (
                    <tr key={provider.id}>
                      <td>{provider.name}</td>
                      <td>{provider.provider_type}</td>
                      <td className="font-mono text-xs">{provider.endpoint || "—"}</td>
                      <td className="text-muted">已通过密钥引用配置</td>
                      <td>{provider.status}</td>
                      {canAdmin ? (
                        <td className="whitespace-nowrap">
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() => {
                              setProviderFormError(null);
                              setEditingProvider(provider);
                              setProviderEditForm({
                                name: provider.name,
                                endpoint: provider.endpoint ?? "",
                                secret_ref: "",
                                status: provider.status,
                              });
                              setProviderEditOpen(true);
                            }}
                          >
                            编辑
                          </button>
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            disabled={!canDeleteProvider(provider)}
                            title={
                              canDeleteProvider(provider)
                                ? undefined
                                : "系统默认 Provider 不可删除"
                            }
                            onClick={() =>
                              setDeleteTarget({
                                kind: "provider",
                                id: provider.id,
                                name: provider.name,
                              })
                            }
                          >
                            删除
                          </button>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}

          <PanelSubhead title="存储池" />
          {pools.length === 0 ? (
            <div className="empty-state">暂无存储池</div>
          ) : (
            <div className="table-wrap">
              <table className="data-table text-sm">
                <thead>
                  <tr>
                    <th>名称</th>
                    <th>状态</th>
                    <th>成员</th>
                    {canAdmin ? <th>操作</th> : null}
                  </tr>
                </thead>
                <tbody>
                  {pools.map((pool) => (
                    <tr key={pool.id}>
                      <td>{pool.name}</td>
                      <td>{pool.status}</td>
                      <td className="text-xs text-muted">
                        {pool.members.length === 0
                          ? "—"
                          : pool.members
                              .map(
                                (member) =>
                                  `${providerNameById.get(member.provider_id) ?? member.provider_id.slice(0, 8)} (${member.mode})`,
                              )
                              .join(", ")}
                      </td>
                      {canAdmin ? (
                        <td className="whitespace-nowrap">
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() => {
                              setPoolFormError(null);
                              setEditingPool(pool);
                              setPoolEditName(pool.name);
                              setPoolEditStatus(pool.status);
                              setPoolEditMembers(() => {
                                const members = membersFromPool(pool);
                                return members.length > 0
                                  ? members
                                  : createDefaultStoragePoolMembers(providers);
                              });
                              setPoolEditOpen(true);
                            }}
                          >
                            编辑
                          </button>
                          <button
                            type="button"
                            className="btn btn-ghost btn-sm"
                            onClick={() =>
                              setDeleteTarget({ kind: "pool", id: pool.id, name: pool.name })
                            }
                          >
                            删除
                          </button>
                        </td>
                      ) : null}
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      <Modal
        open={providerOpen}
        title="注册存储 Provider"
        onClose={() => {
          setProviderFormError(null);
          setProviderOpen(false);
        }}
      >
        <div className="stack-md">
          {providerFormError ? <InlineAlert tone="error">{providerFormError}</InlineAlert> : null}
          <label className="stack-xs">
            <span>名称</span>
            <input
              className="input"
              value={providerForm.name}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderForm((f) => ({ ...f, name: e.target.value }));
              }}
            />
          </label>
          <label className="stack-xs">
            <span>类型</span>
            <select
              className="input"
              value={providerForm.provider_type}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderForm((f) => ({
                  ...f,
                  provider_type: e.target.value as RegisterStorageProviderCommand["provider_type"],
                }));
              }}
            >
              {PROVIDER_TYPES.map((type) => (
                <option key={type} value={type}>
                  {type}
                </option>
              ))}
            </select>
          </label>
          <label className="stack-xs">
            <span>Endpoint（可选）</span>
            <input
              className="input"
              value={providerForm.endpoint ?? ""}
              onChange={(e) => setProviderForm((f) => ({ ...f, endpoint: e.target.value }))}
            />
          </label>
          <label className="stack-xs">
            <span>Secret 引用</span>
            <input
              className="input font-mono text-xs"
              placeholder={registerProviderSecretPlaceholder(providerForm.provider_type)}
              value={providerForm.secret_ref}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderForm((f) => ({ ...f, secret_ref: e.target.value }));
              }}
            />
          </label>
          <ModalFooter
            pending={submitting}
            onCancel={() => setProviderOpen(false)}
            onSubmit={() => void submitProvider()}
            submitLabel="保存"
          />
        </div>
      </Modal>

      <Modal
        open={poolOpen}
        title="新增存储池"
        onClose={() => {
          setPoolFormError(null);
          setPoolOpen(false);
        }}
      >
        <div className="stack-md">
          {poolFormError ? <InlineAlert tone="error">{poolFormError}</InlineAlert> : null}
          <label className="stack-xs">
            <span>名称</span>
            <input
              className="input"
              value={poolName}
              onChange={(e) => {
                setPoolFormError(null);
                setPoolName(e.target.value);
              }}
            />
          </label>
          <StoragePoolMembersEditor
            activeProviders={activeProviders}
            members={poolMembers}
            providerNameById={providerNameById}
            onChange={setPoolMembers}
            onClearError={() => setPoolFormError(null)}
          />
          <ModalFooter
            pending={submitting}
            submitDisabled={activeProviders.length === 0}
            onCancel={() => {
              setPoolFormError(null);
              setPoolOpen(false);
            }}
            onSubmit={() => void submitPool()}
            submitLabel="创建"
          />
        </div>
      </Modal>

      <Modal
        open={providerEditOpen}
        title="编辑存储 Provider"
        onClose={() => {
          setProviderFormError(null);
          setProviderEditOpen(false);
          setEditingProvider(null);
        }}
      >
        <div className="stack-md">
          {providerFormError ? <InlineAlert tone="error">{providerFormError}</InlineAlert> : null}
          <label className="stack-xs">
            <span>名称</span>
            <input
              className="input"
              value={providerEditForm.name}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderEditForm((current) => ({ ...current, name: e.target.value }));
              }}
            />
          </label>
          <label className="stack-xs">
            <span>类型</span>
            <input className="input" value={editingProvider?.provider_type ?? ""} disabled />
          </label>
          <label className="stack-xs">
            <span>Endpoint（可选）</span>
            <input
              className="input"
              value={providerEditForm.endpoint ?? ""}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderEditForm((current) => ({ ...current, endpoint: e.target.value }));
              }}
            />
          </label>
          <label className="stack-xs">
            <span>Secret 引用</span>
            <input
              className="input font-mono text-xs"
              placeholder="留空表示不修改"
              value={providerEditForm.secret_ref ?? ""}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderEditForm((current) => ({ ...current, secret_ref: e.target.value }));
              }}
            />
          </label>
          <label className="stack-xs">
            <span>状态</span>
            <select
              className="input"
              value={providerEditForm.status ?? "ACTIVE"}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderEditForm((current) => ({ ...current, status: e.target.value }));
              }}
            >
              {PROVIDER_STATUSES.map((status) => (
                <option key={status} value={status}>
                  {status}
                </option>
              ))}
            </select>
          </label>
          <ModalFooter
            pending={submitting}
            onCancel={() => setProviderEditOpen(false)}
            onSubmit={() => void submitProviderUpdate()}
            submitLabel="保存"
          />
        </div>
      </Modal>

      <Modal
        open={poolEditOpen}
        title="编辑存储池"
        onClose={() => {
          setPoolFormError(null);
          setPoolEditOpen(false);
          setEditingPool(null);
        }}
      >
        <div className="stack-md">
          {poolFormError ? <InlineAlert tone="error">{poolFormError}</InlineAlert> : null}
          <label className="stack-xs">
            <span>名称</span>
            <input
              className="input"
              value={poolEditName}
              onChange={(e) => {
                setPoolFormError(null);
                setPoolEditName(e.target.value);
              }}
            />
          </label>
          <label className="stack-xs">
            <span>状态</span>
            <select
              className="input"
              value={poolEditStatus}
              onChange={(e) => {
                setPoolFormError(null);
                setPoolEditStatus(e.target.value as StoragePoolVM["status"]);
              }}
            >
              {PROVIDER_STATUSES.map((status) => (
                <option key={status} value={status}>
                  {status}
                </option>
              ))}
            </select>
          </label>
          <StoragePoolMembersEditor
            activeProviders={activeProviders}
            members={poolEditMembers}
            providerNameById={providerNameById}
            onChange={setPoolEditMembers}
            onClearError={() => setPoolFormError(null)}
          />
          <ModalFooter
            pending={submitting}
            submitDisabled={activeProviders.length === 0}
            onCancel={() => setPoolEditOpen(false)}
            onSubmit={() => void submitPoolUpdate()}
            submitLabel="保存"
          />
        </div>
      </Modal>

      <Modal
        open={deleteTarget !== null}
        title="确认删除"
        onClose={() => setDeleteTarget(null)}
      >
        <div className="stack-md">
          <p className="text-sm">
            确定删除{deleteTarget?.kind === "provider" ? " Provider " : "存储池 "}
            <strong>{deleteTarget?.name}</strong> 吗？
            {deleteTarget?.kind === "provider"
              ? " 若仍被存储池引用，将无法删除。"
              : " 若仍被作用域绑定引用，将无法删除。"}
          </p>
          <ModalFooter
            pending={submitting}
            onCancel={() => setDeleteTarget(null)}
            onSubmit={() => void confirmDelete()}
            submitLabel="删除"
          />
        </div>
      </Modal>
    </section>
  );
}

function PanelHeader(props: {
  title: string;
  description: string;
  canAdmin: boolean;
  dialogOpen: boolean;
  onAddProvider: () => void;
  onAddPool: () => void;
}) {
  return (
    <div className="flex flex-wrap items-start justify-between gap-3">
      <div>
        <h2 className="text-lg font-semibold">{props.title}</h2>
        <p className="text-sm text-muted">{props.description}</p>
      </div>
      {props.canAdmin ? (
        <div className="flex flex-wrap gap-2">
          <button type="button" className="btn btn-secondary" onClick={props.onAddProvider}>
            {STORAGE_PROVIDER_REGISTER_LABEL}
          </button>
          <button
            type="button"
            className={props.dialogOpen ? "btn btn-secondary" : "btn btn-primary"}
            onClick={props.onAddPool}
          >
            {STORAGE_POOL_CREATE_LABEL}
          </button>
        </div>
      ) : null}
    </div>
  );
}

function PanelSubhead({ title }: { title: string }) {
  return <h3 className="text-base font-medium">{title}</h3>;
}

function ModalFooter(props: {
  pending: boolean;
  submitDisabled?: boolean;
  onCancel: () => void;
  onSubmit: () => void;
  submitLabel: string;
}) {
  return (
    <div className="flex justify-end gap-2">
      <button type="button" className="btn btn-ghost" disabled={props.pending} onClick={props.onCancel}>
        取消
      </button>
      <button
        type="button"
        className="btn btn-primary"
        disabled={props.pending || props.submitDisabled}
        onClick={props.onSubmit}
      >
        {props.submitLabel}
      </button>
    </div>
  );
}
