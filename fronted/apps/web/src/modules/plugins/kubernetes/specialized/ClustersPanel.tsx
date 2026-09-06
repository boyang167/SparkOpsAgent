"use client";

import { useEffect, useMemo, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { Modal } from "@/components/ui/Modal";
import { notifyEntitlementsUpdated, useShellContext } from "@/components/layout/shell-context";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import {
  ClusterCreateForm,
  EMPTY_CLUSTER_CREATE_FORM,
  formFromCluster,
  type ClusterCreateFormValue,
} from "../ClusterCreateForm";
import {
  clusterStatusBadgeClass,
  clusterStatusFromProbe,
  clusterStatusView,
  clusterVersionProxyPath,
  type ClusterStatusView,
} from "../cluster-status";
import type { KubernetesCluster } from "../cluster-types";
import { ClusterAssociateDialog } from "./ClusterAssociateDialog";
import { ClusterAuthorizeDialog } from "./ClusterAuthorizeDialog";

const PAGE_SIZES = [10, 30, 50];

function settingsPayload(form: ClusterCreateFormValue) {
  return {
    prometheus_url: form.prometheus_url,
    prometheus_user: form.prometheus_auth_type === 2 ? form.prometheus_user : "",
    prometheus_password: form.prometheus_auth_type === 2 ? form.prometheus_password : "",
    velaux_url: form.velaux_url,
    velaux_user: form.velaux_user,
    velaux_password: form.velaux_password,
    llm_base_url: form.llm_base_url,
    llm_api_key: form.llm_api_key,
    llm_model: form.llm_model,
  };
}

export function ClustersPanel({
  clusters,
  loading,
  error,
  canManage,
  selectedId,
  onRefresh,
  onSelect,
  onClustersChange,
}: {
  clusters: KubernetesCluster[];
  loading: boolean;
  error: ApiErrorInfo | null;
  canManage: boolean;
  selectedId: string;
  onRefresh: () => void;
  onSelect: (clusterId: string) => void;
  onClustersChange: (clusters: KubernetesCluster[]) => void;
}) {
  const { workspaceId } = useShellContext();
  const [keyword, setKeyword] = useState("");
  const [draftKeyword, setDraftKeyword] = useState("");
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [selected, setSelected] = useState<string[]>([]);
  const [creating, setCreating] = useState(false);
  const [createForm, setCreateForm] = useState<ClusterCreateFormValue>(EMPTY_CLUSTER_CREATE_FORM);
  const [createSaving, setCreateSaving] = useState(false);
  const [createError, setCreateError] = useState<ApiErrorInfo | null>(null);
  const [changing, setChanging] = useState<KubernetesCluster | null>(null);
  const [changeForm, setChangeForm] = useState<ClusterCreateFormValue>(EMPTY_CLUSTER_CREATE_FORM);
  const [authorizing, setAuthorizing] = useState<KubernetesCluster | null>(null);
  const [associating, setAssociating] = useState<KubernetesCluster | null>(null);
  const [actionError, setActionError] = useState<ApiErrorInfo | null>(null);
  const [statuses, setStatuses] = useState<Record<string, ClusterStatusView>>({});

  const filtered = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    if (!q) return clusters;
    return clusters.filter((item) => `${item.name} ${item.id} ${item.server}`.toLowerCase().includes(q));
  }, [clusters, keyword]);

  const total = filtered.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(page, totalPages);
  const rows = filtered.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const rowIds = rows.map((item) => item.id).join(",");
  const allChecked = rows.length > 0 && rows.every((item) => selected.includes(item.id));

  useEffect(() => {
    if (!rowIds) {
      return;
    }
    const ids = rowIds.split(",");
    let cancelled = false;
    setStatuses((prev) => {
      const next = { ...prev };
      for (const id of ids) {
        next[id] = clusterStatusView("checking");
      }
      return next;
    });
    void Promise.all(
      ids.map(async (id) => {
        try {
          await k8sFetch(clusterVersionProxyPath(id));
          if (!cancelled) {
            setStatuses((prev) => ({ ...prev, [id]: clusterStatusFromProbe(true) }));
          }
        } catch (err) {
          const info = errorInfo(err, "无法连接 Kubernetes API");
          if (!cancelled) {
            setStatuses((prev) => ({ ...prev, [id]: clusterStatusFromProbe(false, info.message) }));
          }
        }
      }),
    );
    return () => {
      cancelled = true;
    };
  }, [rowIds, loading]);

  function toggleAll(checked: boolean) {
    const ids = rows.map((item) => item.id);
    setSelected((prev) => (checked ? Array.from(new Set([...prev, ...ids])) : prev.filter((id) => !ids.includes(id))));
  }

  function toggleOne(id: string, checked: boolean) {
    setSelected((prev) => (checked ? [...prev, id] : prev.filter((item) => item !== id)));
  }

  function openCreate() {
    setCreateForm(EMPTY_CLUSTER_CREATE_FORM);
    setCreateError(null);
    setCreating(true);
  }

  function openChange(cluster: KubernetesCluster) {
    setChanging(cluster);
    setChangeForm(formFromCluster(cluster));
    setCreateError(null);
  }

  async function saveCreate() {
    if (!canManage || !workspaceId) return;
    if (!createForm.name.trim() || !createForm.kube_config.trim() || !createForm.api_address.trim()) {
      setCreateError({ code: "INVALID_REQUEST", message: "请填写名称、集群凭证和接口地址" });
      return;
    }
    setCreateSaving(true);
    setCreateError(null);
    try {
      const created = await platform.registerWorkspaceCluster(workspaceId, {
        cluster_name: createForm.name.trim(),
        api_address: createForm.api_address.trim(),
        server: createForm.api_address.trim(),
        kube_type: createForm.kube_type,
        kube_config: createForm.kube_config,
      });
      const payload = settingsPayload(createForm);
      if (Object.values(payload).some((value) => value.trim() !== "")) {
        await k8sFetch<KubernetesCluster>(
          `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(created.id)}/settings`,
          { method: "PUT", body: JSON.stringify(payload) },
        );
      }
      notifyEntitlementsUpdated();
      setCreating(false);
      onRefresh();
      onSelect(created.id);
    } catch (err) {
      setCreateError(
        err instanceof BffError
          ? { code: err.code || "REGISTER_FAILED", message: err.message }
          : errorInfo(err, "创建集群失败"),
      );
    } finally {
      setCreateSaving(false);
    }
  }

  async function saveChange() {
    if (!changing || !canManage || !workspaceId) return;
    const nextAddress = changeForm.api_address.trim();
    const addressChanged = nextAddress !== (changing.server ?? "").trim();
    const hasCredential = changeForm.kube_config.trim() !== "";
    if (addressChanged && !nextAddress) {
      setCreateError({ code: "INVALID_REQUEST", message: "请填写接口地址" });
      return;
    }
    setCreateSaving(true);
    setCreateError(null);
    try {
      let clusterId = changing.id;
      if (hasCredential || addressChanged) {
        const registerPayload: Parameters<typeof platform.registerWorkspaceCluster>[1] = {
          cluster_name: changeForm.name.trim() || changing.name,
          api_address: nextAddress || changing.server,
          server: nextAddress || changing.server,
        };
        if (hasCredential) {
          registerPayload.kube_type = changeForm.kube_type;
          registerPayload.kube_config = changeForm.kube_config;
        }
        const updated = await platform.registerWorkspaceCluster(workspaceId, registerPayload);
        clusterId = updated.id;
      }
      const updated = await k8sFetch<KubernetesCluster>(
        `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/settings`,
        { method: "PUT", body: JSON.stringify(settingsPayload(changeForm)) },
      );
      onClustersChange(clusters.map((item) => (item.id === updated.id ? updated : item)));
      setChanging(null);
      onRefresh();
    } catch (err) {
      setCreateError(
        err instanceof BffError
          ? { code: err.code || "UPDATE_FAILED", message: err.message }
          : errorInfo(err, "变更集群失败"),
      );
    } finally {
      setCreateSaving(false);
    }
  }

  async function revokeCluster(cluster: KubernetesCluster) {
    if (!canManage || !workspaceId) return;
    if (!window.confirm(`确定撤销集群「${cluster.name || cluster.id}」的工作区授权？平台登记不会被删除。`)) return;
    try {
      await platform.revokeAuthorizedWorkspaceCluster(workspaceId, cluster.id);
      setSelected((prev) => prev.filter((id) => id !== cluster.id));
      notifyEntitlementsUpdated();
      onRefresh();
    } catch (err) {
      setActionError(
        err instanceof BffError
          ? { code: err.code || "REVOKE_FAILED", message: err.message }
          : errorInfo(err, "撤销授权失败"),
      );
    }
  }

  async function revokeSelected() {
    if (!canManage || !workspaceId || selected.length === 0) return;
    if (!window.confirm(`确定批量撤销 ${selected.length} 个集群的工作区授权？`)) return;
    setActionError(null);
    try {
      for (const id of selected) {
        await platform.revokeAuthorizedWorkspaceCluster(workspaceId, id);
      }
      setSelected([]);
      notifyEntitlementsUpdated();
      onRefresh();
    } catch (err) {
      setActionError(
        err instanceof BffError
          ? { code: err.code || "REVOKE_FAILED", message: err.message }
          : errorInfo(err, "批量撤销失败"),
      );
    }
  }

  return (
    <div className="k8s-console stack-md">
      <div className="k8s-search-row">
        <label className="min-w-64">
          <span className="label">名称</span>
          <input
            className="input"
            value={draftKeyword}
            placeholder="名称"
            onChange={(e) => setDraftKeyword(e.target.value)}
          />
        </label>
        <button
          type="button"
          className="btn"
          disabled={loading}
          onClick={() => {
            setKeyword(draftKeyword);
            setPage(1);
            onRefresh();
          }}
        >
          {loading ? "查询中…" : "查询"}
        </button>
        <button
          type="button"
          className="btn-secondary"
          onClick={() => {
            setDraftKeyword("");
            setKeyword("");
            setPage(1);
          }}
        >
          重置
        </button>
      </div>
      {canManage ? (
        <div className="k8s-ops-row">
          <button type="button" className="btn" onClick={openCreate}>
            + 新增
          </button>
          <button type="button" className="btn-secondary" disabled={selected.length === 0} onClick={() => void revokeSelected()}>
            批量删除
          </button>
        </div>
      ) : null}
      <ApiErrorBanner error={error} />
      <ApiErrorBanner error={actionError} />
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>
                <input type="checkbox" checked={allChecked} onChange={(e) => toggleAll(e.target.checked)} aria-label="全选" />
              </th>
              <th>ID</th>
              <th>名称</th>
              <th>状态</th>
              <th>Prometheus</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((item) => {
              const status = statuses[item.id] ?? clusterStatusView("checking");
              return (
              <tr key={item.id} className={item.id === selectedId ? "k8s-row-selected" : undefined}>
                <td>
                  <input
                    type="checkbox"
                    checked={selected.includes(item.id)}
                    onChange={(e) => toggleOne(item.id, e.target.checked)}
                    aria-label={`选择 ${item.name || item.id}`}
                  />
                </td>
                <td className="font-mono text-xs">{item.id}</td>
                <td>
                  <button type="button" className="k8s-action-link" onClick={() => onSelect(item.id)}>
                    {item.name || item.id}
                  </button>
                </td>
                <td>
                  <span className={clusterStatusBadgeClass(status.kind)} title={status.detail}>
                    {status.label}
                  </span>
                </td>
                <td>{item.prometheus_url || "-"}</td>
                <td>
                  {canManage ? (
                    <div className="flex flex-wrap gap-3">
                      <button type="button" className="k8s-action-link" onClick={() => setAuthorizing(item)}>
                        授权
                      </button>
                      <button type="button" className="k8s-action-link" onClick={() => setAssociating(item)}>
                        关联
                      </button>
                      <button type="button" className="k8s-action-link" onClick={() => openChange(item)}>
                        变更
                      </button>
                      <button type="button" className="k8s-action-link" onClick={() => void revokeCluster(item)}>
                        删除
                      </button>
                    </div>
                  ) : (
                    <span className="text-muted">只读</span>
                  )}
                </td>
              </tr>
              );
            })}
            {rows.length === 0 ? (
              <tr>
                <td colSpan={6}>
                  <div className="empty-state">{loading ? "加载集群中…" : "暂无集群，点击「新增」接入"}</div>
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <div className="k8s-pagination">
        <span className="text-muted">共 {total} 条</span>
        <label className="flex items-center gap-2 text-sm">
          <select
            className="input"
            value={pageSize}
            onChange={(e) => {
              setPageSize(Number(e.target.value));
              setPage(1);
            }}
          >
            {PAGE_SIZES.map((size) => (
              <option key={size} value={size}>
                {size}条/页
              </option>
            ))}
          </select>
        </label>
        <div className="flex items-center gap-2">
          <button type="button" className="btn-secondary text-sm" disabled={currentPage <= 1} onClick={() => setPage((p) => p - 1)}>
            上一页
          </button>
          <span className="k8s-page-current">{currentPage}</span>
          <button
            type="button"
            className="btn-secondary text-sm"
            disabled={currentPage >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            下一页
          </button>
        </div>
        <label className="flex items-center gap-1 text-sm text-muted">
          前往
          <input
            className="input k8s-page-jumper"
            type="number"
            min={1}
            max={totalPages}
            value={currentPage}
            onChange={(e) => setPage(Math.min(totalPages, Math.max(1, Number(e.target.value) || 1)))}
          />
          页
        </label>
      </div>

      <Modal
        open={creating}
        title="创建"
        size="xl"
        onClose={() => setCreating(false)}
        footer={
          <>
            <button type="button" className="btn-secondary" onClick={() => setCreating(false)}>
              取消
            </button>
            <button type="button" className="btn" disabled={createSaving} onClick={() => void saveCreate()}>
              {createSaving ? "保存中…" : "确定"}
            </button>
          </>
        }
      >
        <ApiErrorBanner error={createError} />
        <ClusterCreateForm value={createForm} onChange={setCreateForm} />
      </Modal>

      <Modal
        open={Boolean(changing)}
        title={changing ? `变更 ${changing.name || changing.id}` : "变更"}
        size="xl"
        onClose={() => setChanging(null)}
        footer={
          <>
            <button type="button" className="btn-secondary" onClick={() => setChanging(null)}>
              取消
            </button>
            <button type="button" className="btn" disabled={createSaving} onClick={() => void saveChange()}>
              {createSaving ? "保存中…" : "确定"}
            </button>
          </>
        }
      >
        <p className="text-sm text-muted mb-3">密钥字段为 *** 时可直接保存以保留原值。修改接口地址可直接保存；如需更新凭证请重新粘贴。</p>
        <ApiErrorBanner error={createError} />
        <ClusterCreateForm value={changeForm} onChange={setChangeForm} nameReadOnly credentialOptional />
      </Modal>

      <Modal open={Boolean(authorizing)} title={authorizing ? `授权 ${authorizing.name || authorizing.id}` : "授权"} size="xl" onClose={() => setAuthorizing(null)}>
        {authorizing ? <ClusterAuthorizeDialog cluster={authorizing} canManage={canManage} /> : null}
      </Modal>

      <Modal open={Boolean(associating)} title={associating ? `关联 ${associating.name || associating.id}` : "关联"} size="xl" onClose={() => setAssociating(null)}>
        {associating ? <ClusterAssociateDialog cluster={associating} /> : null}
      </Modal>
    </div>
  );
}
