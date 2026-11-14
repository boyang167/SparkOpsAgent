"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import type {
  CreateStoragePoolCommand,
  RegisterStorageProviderCommand,
  StoragePoolVM,
  StorageProviderVM,
} from "@sparkops/contracts/filesystem";
import { BffError } from "@sparkops/contracts";
import {
  createStoragePool,
  listStoragePools,
  listStorageProviders,
  registerStorageProvider,
} from "@sparkops/web-sdk/filesystem";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import {
  canShowStoragePoolMutations,
  providerRowsForDisplay,
  STORAGE_POOL_CREATE_LABEL,
  STORAGE_PROVIDER_REGISTER_LABEL,
} from "./storage-pool-panel-view";

export type StoragePoolPanelProps = {
  workspaceId: string;
  permissions: string[];
  providers?: StorageProviderVM[];
  pools?: StoragePoolVM[];
  onReload?: () => void;
};

const PROVIDER_TYPES = ["LOCAL", "NFS", "S3"] as const;

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
  const [providerForm, setProviderForm] = useState<RegisterStorageProviderCommand>({
    name: "",
    provider_type: "S3",
    secret_ref: "",
    endpoint: "",
  });
  const [poolName, setPoolName] = useState("");

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

  const providerRows = useMemo(() => providerRowsForDisplay(providers), [providers]);

  async function submitProvider() {
    if (!canAdmin || !providerForm.name.trim() || !providerForm.secret_ref.trim()) return;
    setSubmitting(true);
    setError(null);
    try {
      await registerStorageProvider(workspaceId, {
        ...providerForm,
        name: providerForm.name.trim(),
        secret_ref: providerForm.secret_ref.trim(),
        endpoint: providerForm.endpoint?.trim() || undefined,
      });
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
    if (!canAdmin || !poolName.trim()) return;
    setSubmitting(true);
    setError(null);
    try {
      const input: CreateStoragePoolCommand = { name: poolName.trim(), members: [] };
      await createStoragePool(workspaceId, input);
      setPoolOpen(false);
      setPoolName("");
      await load();
      onReload?.();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建存储池失败");
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <section className="stack-lg">
      <PanelHeader
        title="存储 Provider 与存储池"
        description="凭证仅通过 Secret 引用提交，界面不回显任何密钥内容。"
        canAdmin={canAdmin}
        dialogOpen={providerOpen || poolOpen}
        onAddProvider={() => setProviderOpen(true)}
        onAddPool={() => setPoolOpen(true)}
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
                  </tr>
                </thead>
                <tbody>
                  {providerRows.map((row) => (
                    <tr key={row.id}>
                      <td>{row.name}</td>
                      <td>{row.provider_type}</td>
                      <td className="font-mono text-xs">{row.endpoint}</td>
                      <td className="text-muted">{row.credential}</td>
                      <td>{row.status}</td>
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
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}

      <Modal open={providerOpen} title="注册存储 Provider" onClose={() => setProviderOpen(false)}>
        <div className="stack-md">
          <label className="stack-xs">
            <span>名称</span>
            <input
              className="input"
              value={providerForm.name}
              onChange={(e) => setProviderForm((f) => ({ ...f, name: e.target.value }))}
            />
          </label>
          <label className="stack-xs">
            <span>类型</span>
            <select
              className="input"
              value={providerForm.provider_type}
              onChange={(e) =>
                setProviderForm((f) => ({
                  ...f,
                  provider_type: e.target.value as RegisterStorageProviderCommand["provider_type"],
                }))
              }
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
              placeholder="secret://..."
              value={providerForm.secret_ref}
              onChange={(e) => setProviderForm((f) => ({ ...f, secret_ref: e.target.value }))}
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

      <Modal open={poolOpen} title="新增存储池" onClose={() => setPoolOpen(false)}>
        <div className="stack-md">
          <label className="stack-xs">
            <span>名称</span>
            <input className="input" value={poolName} onChange={(e) => setPoolName(e.target.value)} />
          </label>
          <ModalFooter
            pending={submitting}
            onCancel={() => setPoolOpen(false)}
            onSubmit={() => void submitPool()}
            submitLabel="创建"
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
  onCancel: () => void;
  onSubmit: () => void;
  submitLabel: string;
}) {
  return (
    <div className="flex justify-end gap-2">
      <button type="button" className="btn btn-ghost" disabled={props.pending} onClick={props.onCancel}>
        取消
      </button>
      <button type="button" className="btn btn-primary" disabled={props.pending} onClick={props.onSubmit}>
        {props.submitLabel}
      </button>
    </div>
  );
}
