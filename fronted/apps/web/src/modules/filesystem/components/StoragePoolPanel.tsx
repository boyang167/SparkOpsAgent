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
import { StorageProviderCardGrid } from "./StorageProviderCardGrid";
import { StoragePoolModalFooter, StoragePoolModalForm } from "./StoragePoolModalForm";
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

export type StoragePoolPanelFocus = "providers" | "pools";

export type StoragePoolPanelProps = {
  workspaceId: string;
  permissions: string[];
  /** 单页只展示 Provider 或存储池；不传则保留合并视图（兼容旧入口） */
  focus?: StoragePoolPanelFocus;
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
  focus,
  providers: initialProviders,
  pools: initialPools,
  onReload,
}: StoragePoolPanelProps) {
  const showProviders = focus !== "pools";
  const showPools = focus !== "providers";
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
      const loadProviders = showProviders || showPools;
      const loadPools = showPools || focus === "providers";
      const [providerPage, poolPage] = await Promise.all([
        loadProviders ? listStorageProviders(workspaceId) : Promise.resolve({ items: [] }),
        loadPools ? listStoragePools(workspaceId) : Promise.resolve({ items: [] }),
      ]);
      if (loadProviders) setProviders(providerPage.items);
      if (loadPools) setPools(poolPage.items);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载存储配置失败");
    } finally {
      setLoading(false);
    }
  }, [focus, showPools, showProviders, workspaceId]);

  function beginEditProvider(provider: StorageProviderVM) {
    setProviderFormError(null);
    setEditingProvider(provider);
    setProviderEditForm({
      name: provider.name,
      endpoint: provider.endpoint ?? "",
      secret_ref: "",
      status: provider.status,
    });
    setProviderEditOpen(true);
  }

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
    const validationError = validateUpdateProviderForm(
      providerEditForm,
      editingProvider.provider_type,
    );
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
        title={
          focus === "providers" || focus === "pools"
            ? undefined
            : "存储 Provider 与存储池"
        }
        canAdmin={canAdmin}
        dialogOpen={dialogOpen}
        showProviderActions={showProviders}
        showPoolActions={showPools}
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
          {showProviders && !focus ? <PanelSubhead title="Provider" /> : null}
          {showProviders && providers.length === 0 ? (
            <div className="empty-state">尚未注册 Provider</div>
          ) : null}
          {showProviders && providers.length > 0 ? (
            <StorageProviderCardGrid
              providers={providers}
              pools={pools}
              canAdmin={canAdmin}
              canDelete={canDeleteProvider}
              onEdit={beginEditProvider}
              onDelete={(provider) =>
                setDeleteTarget({
                  kind: "provider",
                  id: provider.id,
                  name: provider.name,
                })
              }
            />
          ) : null}

          {showPools && !focus ? <PanelSubhead title="存储池" /> : null}
          {showPools && pools.length === 0 ? (
            <div className="empty-state">暂无存储池</div>
          ) : null}
          {showPools && pools.length > 0 ? (
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
          ) : null}
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
            <span>Endpoint</span>
            <input
              className="input"
              placeholder={
                providerForm.provider_type === "LOCAL" || providerForm.provider_type === "NFS"
                  ? "例如 A:\\data\\filesystem（必须是已存在的目录）"
                  : "例如 https://minio.example.com"
              }
              value={providerForm.endpoint ?? ""}
              onChange={(e) => {
                setProviderFormError(null);
                setProviderForm((f) => ({ ...f, endpoint: e.target.value }));
              }}
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
        size="lg"
        panelClassName="filesystem-pool-modal"
        onClose={() => {
          setPoolFormError(null);
          setPoolOpen(false);
        }}
        footer={
          <StoragePoolModalFooter
            pending={submitting}
            submitDisabled={activeProviders.length === 0}
            onCancel={() => {
              setPoolFormError(null);
              setPoolOpen(false);
            }}
            onSubmit={() => void submitPool()}
            submitLabel="创建"
          />
        }
      >
        <StoragePoolModalForm
          error={poolFormError}
          name={poolName}
          showStatus={false}
          onNameChange={setPoolName}
          activeProviders={activeProviders}
          members={poolMembers}
          onMembersChange={setPoolMembers}
          providerNameById={providerNameById}
          onClearError={() => setPoolFormError(null)}
        />
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
            <span>Endpoint</span>
            <input
              className="input"
              placeholder="必须是已存在的本地目录或可访问的 S3 地址"
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
        size="lg"
        panelClassName="filesystem-pool-modal"
        onClose={() => {
          setPoolFormError(null);
          setPoolEditOpen(false);
          setEditingPool(null);
        }}
        footer={
          <StoragePoolModalFooter
            pending={submitting}
            submitDisabled={activeProviders.length === 0}
            onCancel={() => {
              setPoolFormError(null);
              setPoolEditOpen(false);
              setEditingPool(null);
            }}
            onSubmit={() => void submitPoolUpdate()}
            submitLabel="保存"
          />
        }
      >
        <StoragePoolModalForm
          error={poolFormError}
          name={poolEditName}
          status={poolEditStatus}
          onNameChange={setPoolEditName}
          onStatusChange={(value) => setPoolEditStatus(value as StoragePoolVM["status"])}
          activeProviders={activeProviders}
          members={poolEditMembers}
          onMembersChange={setPoolEditMembers}
          providerNameById={providerNameById}
          onClearError={() => setPoolFormError(null)}
        />
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
  title?: string;
  canAdmin: boolean;
  dialogOpen: boolean;
  showProviderActions: boolean;
  showPoolActions: boolean;
  onAddProvider: () => void;
  onAddPool: () => void;
}) {
  return (
    <div
      className={
        props.title
          ? "flex flex-wrap items-start justify-between gap-3"
          : "flex flex-wrap items-start justify-end gap-3"
      }
    >
      {props.title ? (
        <div>
          <h2 className="text-lg font-semibold">{props.title}</h2>
        </div>
      ) : null}
      {props.canAdmin ? (
        <div className="flex flex-wrap gap-2">
          {props.showProviderActions ? (
            <button
              type="button"
              className={
                props.showPoolActions || props.dialogOpen ? "btn btn-secondary" : "btn btn-primary"
              }
              onClick={props.onAddProvider}
            >
              {STORAGE_PROVIDER_REGISTER_LABEL}
            </button>
          ) : null}
          {props.showPoolActions ? (
            <button
              type="button"
              className={props.dialogOpen ? "btn btn-secondary" : "btn btn-primary"}
              onClick={props.onAddPool}
            >
              {STORAGE_POOL_CREATE_LABEL}
            </button>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function PanelSubhead({ title }: { title?: string }) {
  if (!title) return null;
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
