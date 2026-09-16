"use client";

import { useEffect, useMemo, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import {
  K8sEmptyState,
  K8sLoadingState,
  K8sPagination,
  K8sPanelCard,
  K8sStatusBadge,
  useK8sConfirm,
} from "@/components/k8s";
import type { K8sStatusTone } from "@/components/k8s/K8sStatusBadge";
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
  clusterStatusFromProbe,
  clusterStatusView,
  clusterVersionProxyPath,
  type ClusterStatusView,
} from "../cluster-status";
import type { KubernetesCluster } from "../cluster-types";
import {
  clusterIntegrationLabel,
  clusterIntegrations,
  formatClusterServer,
  truncateClusterId,
} from "../cluster-view";
import { ClusterAssociateDialog } from "./ClusterAssociateDialog";
import { ClusterAuthorizeDialog } from "./ClusterAuthorizeDialog";

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

function statusTone(status: ClusterStatusView): K8sStatusTone {
  if (status.kind === "healthy") return "success";
  if (status.kind === "unhealthy") return "danger";
  return "info";
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
  const [moreCluster, setMoreCluster] = useState<KubernetesCluster | null>(null);
  const [actionError, setActionError] = useState<ApiErrorInfo | null>(null);
  const [statuses, setStatuses] = useState<Record<string, ClusterStatusView>>({});
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const filtered = useMemo(() => {
    const q = keyword.trim().toLowerCase();
    if (!q) return clusters;
    return clusters.filter((item) => `${item.name} ${item.server} ${item.id}`.toLowerCase().includes(q));
  }, [clusters, keyword]);

  const total = filtered.length;
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const currentPage = Math.min(page, totalPages);
  const rows = filtered.slice((currentPage - 1) * pageSize, currentPage * pageSize);
  const rowIds = rows.map((item) => item.id).join(",");
  const allChecked = rows.length > 0 && rows.every((item) => selected.includes(item.id));

  const statusSummary = useMemo(() => {
    let healthy = 0;
    let unhealthy = 0;
    let checking = 0;
    for (const item of filtered) {
      const status = statuses[item.id] ?? clusterStatusView("checking");
      if (status.kind === "healthy") healthy += 1;
      else if (status.kind === "unhealthy") unhealthy += 1;
      else checking += 1;
    }
    return { healthy, unhealthy, checking };
  }, [filtered, statuses]);

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

  function applySearch() {
    setKeyword(draftKeyword);
    setPage(1);
    onRefresh();
  }

  function resetSearch() {
    setDraftKeyword("");
    setKeyword("");
    setPage(1);
  }

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
    setMoreCluster(null);
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
    const ok = await ask({
      title: "撤销集群授权",
      description: `确定撤销集群「${cluster.name || cluster.id}」的工作区授权？平台登记不会被删除。`,
      confirmLabel: "撤销",
      danger: true,
    });
    if (!ok) return;
    try {
      await platform.revokeAuthorizedWorkspaceCluster(workspaceId, cluster.id);
      setSelected((prev) => prev.filter((id) => id !== cluster.id));
      setMoreCluster(null);
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
    const ok = await ask({
      title: "批量撤销授权",
      description: `确定批量撤销 ${selected.length} 个集群的工作区授权？`,
      confirmLabel: "撤销",
      danger: true,
    });
    if (!ok) return;
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

  async function copyClusterId(id: string) {
    try {
      await navigator.clipboard.writeText(id);
    } catch {
      setActionError({ code: "CLIPBOARD_FAILED", message: "无法复制集群 ID，请手动选择复制" });
    }
  }

  return (
    <K8sPanelCard>
      <div className="k8s-toolbar-row">
        <label className="min-w-64 flex-1">
          <span className="label">搜索</span>
          <input
            className="input"
            value={draftKeyword}
            placeholder="名称或接口地址"
            onChange={(e) => setDraftKeyword(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") applySearch();
            }}
          />
        </label>
        <button type="button" className="btn" disabled={loading} onClick={applySearch}>
          {loading ? "查询中…" : "查询"}
        </button>
        <button type="button" className="btn-secondary" onClick={resetSearch}>
          重置
        </button>
        {canManage ? (
          <div className="k8s-toolbar-actions">
            <button type="button" className="btn" onClick={openCreate}>
              接入集群
            </button>
            <button
              type="button"
              className="btn-secondary"
              disabled={selected.length === 0}
              onClick={() => void revokeSelected()}
            >
              批量撤销{selected.length > 0 ? ` (${selected.length})` : ""}
            </button>
          </div>
        ) : null}
      </div>

      {!loading && total > 0 ? (
        <p className="k8s-cluster-summary" role="status">
          共 {total} 个集群
          {statusSummary.healthy > 0 ? ` · ${statusSummary.healthy} 正常` : ""}
          {statusSummary.unhealthy > 0 ? ` · ${statusSummary.unhealthy} 异常` : ""}
          {statusSummary.checking > 0 ? ` · ${statusSummary.checking} 检测中` : ""}
        </p>
      ) : null}

      <ApiErrorBanner error={error} />
      <ApiErrorBanner error={actionError} />

      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              {canManage ? (
                <th className="w-10">
                  <input type="checkbox" checked={allChecked} onChange={(e) => toggleAll(e.target.checked)} aria-label="全选" />
                </th>
              ) : null}
              <th>集群</th>
              <th>接口地址</th>
              <th>连通性</th>
              <th>集成</th>
              <th className="w-28">操作</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((item) => {
              const status = statuses[item.id] ?? clusterStatusView("checking");
              const integrations = clusterIntegrations(item);
              const isActive = item.id === selectedId;
              return (
                <tr key={item.id} className={isActive ? "k8s-row-selected" : undefined}>
                  {canManage ? (
                    <td>
                      <input
                        type="checkbox"
                        checked={selected.includes(item.id)}
                        onChange={(e) => toggleOne(item.id, e.target.checked)}
                        aria-label={`选择 ${item.name || item.id}`}
                      />
                    </td>
                  ) : null}
                  <td>
                    <button type="button" className="k8s-action-link font-semibold" onClick={() => onSelect(item.id)}>
                      {item.name || truncateClusterId(item.id)}
                    </button>
                    <div className="k8s-cluster-meta">
                      <span className="font-mono text-xs text-muted" title={item.id}>
                        {truncateClusterId(item.id)}
                      </span>
                      <button
                        type="button"
                        className="k8s-cluster-copy-id"
                        aria-label={`复制集群 ID ${item.id}`}
                        onClick={() => void copyClusterId(item.id)}
                      >
                        复制 ID
                      </button>
                    </div>
                  </td>
                  <td className="max-w-xs">
                    <span className="font-mono text-xs break-all">{formatClusterServer(item.server)}</span>
                  </td>
                  <td>
                    <K8sStatusBadge tone={statusTone(status)} dot={status.kind === "checking"} title={status.detail}>
                      {status.label}
                    </K8sStatusBadge>
                  </td>
                  <td>
                    {integrations.length > 0 ? (
                      <div className="k8s-role-badges">
                        {integrations.map((key) => (
                          <span key={key} className="badge badge-neutral">
                            {clusterIntegrationLabel(key)}
                          </span>
                        ))}
                      </div>
                    ) : (
                      <span className="text-muted text-xs">未配置</span>
                    )}
                  </td>
                  <td className="whitespace-nowrap">
                    {canManage ? (
                      <>
                        <button type="button" className="k8s-action-link" onClick={() => openChange(item)}>
                          变更
                        </button>
                        <span className="text-muted"> / </span>
                        <button type="button" className="k8s-action-link" onClick={() => setMoreCluster(item)}>
                          更多
                        </button>
                      </>
                    ) : (
                      <span className="text-muted text-xs">只读</span>
                    )}
                  </td>
                </tr>
              );
            })}
            {loading && rows.length === 0 ? (
              <tr>
                <td colSpan={canManage ? 6 : 5}>
                  <K8sLoadingState label="加载集群中…" compact />
                </td>
              </tr>
            ) : null}
            {!loading && rows.length === 0 ? (
              <tr>
                <td colSpan={canManage ? 6 : 5}>
                  <K8sEmptyState
                    variant="empty"
                    title="暂无集群"
                    description="接入第一个 Kubernetes 集群，即可在此管理工作区内的集群授权与集成配置。"
                    compact
                    action={
                      canManage ? (
                        <button type="button" className="btn" onClick={openCreate}>
                          接入集群
                        </button>
                      ) : undefined
                    }
                  />
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>

      <K8sPagination
        page={currentPage}
        pageSize={pageSize}
        total={total}
        pageSizes={[10, 30, 50]}
        loading={loading}
        onPageChange={setPage}
        onPageSizeChange={(next) => {
          setPageSize(next);
          setPage(1);
        }}
      />

      <Modal
        open={creating}
        title="接入集群"
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

      <Modal
        open={Boolean(moreCluster)}
        title={moreCluster ? `更多操作 · ${moreCluster.name || moreCluster.id}` : "更多操作"}
        description={moreCluster ? formatClusterServer(moreCluster.server) : undefined}
        size="md"
        onClose={() => setMoreCluster(null)}
      >
        {moreCluster ? (
          <div className="k8s-action-sheet">
            <button
              type="button"
              className="k8s-action-sheet-item"
              onClick={() => {
                setAuthorizing(moreCluster);
                setMoreCluster(null);
              }}
            >
              <span className="k8s-action-sheet-label">授权</span>
              <span className="k8s-action-sheet-hint">为成员签发 kubeconfig 或管理集群访问角色</span>
            </button>
            <button
              type="button"
              className="k8s-action-sheet-item"
              onClick={() => {
                setAssociating(moreCluster);
                setMoreCluster(null);
              }}
            >
              <span className="k8s-action-sheet-label">关联</span>
              <span className="k8s-action-sheet-hint">将集群关联到 Velaux 多集群管理</span>
            </button>
            <button type="button" className="k8s-action-sheet-item" onClick={() => openChange(moreCluster)}>
              <span className="k8s-action-sheet-label">变更配置</span>
              <span className="k8s-action-sheet-hint">更新凭证、Prometheus、Velaux 与 LLM 设置</span>
            </button>
            <button
              type="button"
              className="k8s-action-sheet-item is-danger"
              onClick={() => void revokeCluster(moreCluster)}
            >
              <span className="k8s-action-sheet-label">撤销授权</span>
              <span className="k8s-action-sheet-hint">从当前工作区移除该集群，不删除平台登记</span>
            </button>
          </div>
        ) : null}
      </Modal>

      <Modal open={Boolean(authorizing)} title={authorizing ? `授权 ${authorizing.name || authorizing.id}` : "授权"} size="xl" onClose={() => setAuthorizing(null)}>
        {authorizing ? <ClusterAuthorizeDialog cluster={authorizing} canManage={canManage} /> : null}
      </Modal>

      <Modal open={Boolean(associating)} title={associating ? `关联 ${associating.name || associating.id}` : "关联"} size="xl" onClose={() => setAssociating(null)}>
        {associating ? <ClusterAssociateDialog cluster={associating} /> : null}
      </Modal>
      {confirmDialog}
    </K8sPanelCard>
  );
}
