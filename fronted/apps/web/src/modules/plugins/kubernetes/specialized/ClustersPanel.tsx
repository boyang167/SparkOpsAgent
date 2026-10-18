"use client";

import { useCallback, useEffect, useMemo, useState, type RefObject } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import {
  K8sActionSheet,
  K8sActionSheetGroup,
  K8sActionSheetItem,
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
  clusterSettingsPayload,
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

function statusTone(status: ClusterStatusView): K8sStatusTone {
  if (status.kind === "healthy") return "success";
  if (status.kind === "unhealthy") return "danger";
  return "info";
}

function SearchIcon() {
  return (
    <svg className="projects-search-icon" viewBox="0 0 20 20" fill="none" aria-hidden="true">
      <path
        d="M9 3.5a5.5 5.5 0 1 1 0 11 5.5 5.5 0 0 1 0-11Z"
        stroke="currentColor"
        strokeWidth="1.5"
      />
      <path d="M13.5 13.5 17 17" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
    </svg>
  );
}

export function ClustersPanel({
  clusters,
  loading,
  error,
  canManage,
  selectedId,
  onRefresh,
  onSelect,
  createActionRef,
  onClustersChange,
}: {
  clusters: KubernetesCluster[];
  loading: boolean;
  error: ApiErrorInfo | null;
  canManage: boolean;
  selectedId: string;
  onRefresh: () => void;
  onSelect: (clusterId: string) => void;
  createActionRef?: RefObject<(() => void) | null>;
  onClustersChange: (clusters: KubernetesCluster[]) => void;
}) {
  const { workspaceId } = useShellContext();
  const [keyword, setKeyword] = useState("");
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

  function resetSearch() {
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

  const openCreate = useCallback(() => {
    setCreateForm(EMPTY_CLUSTER_CREATE_FORM);
    setCreateError(null);
    setCreating(true);
  }, []);

  useEffect(() => {
    if (!createActionRef) return;
    createActionRef.current = canManage ? openCreate : null;
    return () => {
      createActionRef.current = null;
    };
  }, [canManage, createActionRef, openCreate]);

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
      const payload = clusterSettingsPayload(createForm);
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
        { method: "PUT", body: JSON.stringify(clusterSettingsPayload(changeForm)) },
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

  const hasActiveFilters = Boolean(keyword.trim());

  return (
    <K8sPanelCard>
      <div className="k8s-clusters-toolbar">
        <div className="k8s-clusters-toolbar-filters">
          <label className="projects-search-wrap">
            <SearchIcon />
            <input
              className="projects-search-input"
              value={keyword}
              placeholder="名称或接口地址"
              aria-label="搜索集群"
              onChange={(e) => {
                setKeyword(e.target.value);
                setPage(1);
              }}
            />
          </label>
          {hasActiveFilters ? (
            <button type="button" className="btn-secondary projects-reset-btn" onClick={resetSearch}>
              重置
            </button>
          ) : null}
        </div>
        <div className="k8s-clusters-toolbar-meta">
          {!loading && total > 0 ? (
            <span className="k8s-clusters-count" role="status">
              共 {total} 个集群
              {statusSummary.healthy > 0 ? ` · ${statusSummary.healthy} 正常` : ""}
              {statusSummary.unhealthy > 0 ? ` · ${statusSummary.unhealthy} 异常` : ""}
              {statusSummary.checking > 0 ? ` · ${statusSummary.checking} 检测中` : ""}
            </span>
          ) : null}
          {canManage && selected.length > 0 ? (
            <button type="button" className="btn-secondary text-sm" onClick={() => void revokeSelected()}>
              批量撤销 ({selected.length})
            </button>
          ) : null}
          <button
            type="button"
            className="btn-secondary k8s-clusters-refresh-btn"
            disabled={loading}
            onClick={() => onRefresh()}
            aria-label="刷新集群列表"
          >
            ↻
          </button>
        </div>
      </div>

      <ApiErrorBanner error={error} />
      <ApiErrorBanner error={actionError} />

      <div className="table-wrap k8s-clusters-table">
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
                    <button type="button" className="k8s-cluster-name-link" onClick={() => onSelect(item.id)}>
                      {item.name || truncateClusterId(item.id)}
                    </button>
                    <div className="k8s-cluster-meta">
                      <span className="font-mono text-xs text-muted" title={item.id}>
                        {truncateClusterId(item.id)}
                      </span>
                      <button
                        type="button"
                        className="btn-ghost k8s-cluster-copy-id"
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
                      <span className="text-xs text-muted">
                        {integrations.map((key) => clusterIntegrationLabel(key)).join("、")}
                      </span>
                    ) : (
                      <span className="text-muted text-xs">未配置</span>
                    )}
                  </td>
                  <td className="whitespace-nowrap">
                    {canManage ? (
                      <div className="k8s-row-actions">
                        <button type="button" className="btn-ghost text-sm" onClick={() => openChange(item)}>
                          变更
                        </button>
                        <button type="button" className="btn-ghost text-sm" onClick={() => setMoreCluster(item)}>
                          更多
                        </button>
                      </div>
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
                        <button type="button" className="btn-secondary" onClick={openCreate}>
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
        panelClassName="k8s-cluster-form-modal"
        onClose={() => setCreating(false)}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={() => setCreating(false)}>
              取消
            </button>
            <button type="button" className="btn" disabled={createSaving} onClick={() => void saveCreate()}>
              {createSaving ? "保存中…" : "接入"}
            </button>
          </div>
        }
      >
        <ApiErrorBanner error={createError} />
        <ClusterCreateForm value={createForm} onChange={setCreateForm} />
      </Modal>

      <Modal
        open={Boolean(changing)}
        title={changing ? `变更 ${changing.name || changing.id}` : "变更"}
        size="xl"
        panelClassName="k8s-cluster-form-modal"
        onClose={() => setChanging(null)}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={() => setChanging(null)}>
              取消
            </button>
            <button type="button" className="btn" disabled={createSaving} onClick={() => void saveChange()}>
              {createSaving ? "保存中…" : "保存"}
            </button>
          </div>
        }
      >
        <ApiErrorBanner error={createError} />
        {changing ? (
          <ClusterCreateForm
            key={changing.id}
            value={changeForm}
            onChange={setChangeForm}
            nameReadOnly
            credentialOptional
          />
        ) : null}
      </Modal>

      <Modal
        open={Boolean(moreCluster)}
        title={moreCluster ? moreCluster.name || moreCluster.id : "更多操作"}
        description={moreCluster ? formatClusterServer(moreCluster.server) : undefined}
        size="sm"
        panelClassName="k8s-cluster-dialog-modal k8s-action-sheet-modal"
        onClose={() => setMoreCluster(null)}
      >
        {moreCluster ? (
          <K8sActionSheet>
            <K8sActionSheetGroup>
            <K8sActionSheetItem
              label="授权"
              icon={
                <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                  <path d="M10 11.5a2.5 2.5 0 1 0 0-5 2.5 2.5 0 0 0 0 5Z" stroke="currentColor" strokeWidth="1.5" />
                  <path d="M5 10a5 5 0 0 1 10 0v1.5c0 .8.3 1.6.8 2.2L15 15H5l-.8-1.3c.5-.6.8-1.4.8-2.2V10Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
                </svg>
              }
              onClick={() => {
                setAuthorizing(moreCluster);
                setMoreCluster(null);
              }}
            />
            <K8sActionSheetItem
              label="Velaux 关联"
              icon={
                <svg viewBox="0 0 20 20" fill="none" aria-hidden="true">
                  <path d="M7 10h6M10 7v6" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
                  <path d="M4 6.5 10 3.5l6 3v7l-6 3-6-3v-7Z" stroke="currentColor" strokeWidth="1.5" strokeLinejoin="round" />
                </svg>
              }
              onClick={() => {
                setAssociating(moreCluster);
                setMoreCluster(null);
              }}
            />
            </K8sActionSheetGroup>
          </K8sActionSheet>
        ) : null}
      </Modal>

      <Modal
        open={Boolean(authorizing)}
        title={authorizing ? `授权 · ${authorizing.name || authorizing.id}` : "授权"}
        size="xl"
        panelClassName="k8s-cluster-dialog-modal"
        onClose={() => setAuthorizing(null)}
      >
        {authorizing ? <ClusterAuthorizeDialog cluster={authorizing} canManage={canManage} /> : null}
      </Modal>

      <Modal
        open={Boolean(associating)}
        title={associating ? `Velaux · ${associating.name || associating.id}` : "Velaux"}
        size="xl"
        panelClassName="k8s-cluster-dialog-modal"
        onClose={() => setAssociating(null)}
      >
        {associating ? <ClusterAssociateDialog cluster={associating} /> : null}
      </Modal>
      {confirmDialog}
    </K8sPanelCard>
  );
}
