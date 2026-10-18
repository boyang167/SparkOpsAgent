"use client";

import { useCallback, useEffect, useState } from "react";
import { K8sEmptyState, K8sPagination, useK8sConfirm } from "@/components/k8s";
import { Modal } from "@/components/ui/Modal";
import { ApiErrorBanner } from "../ApiErrorBanner";
import { errorInfo, k8sFetch, type ApiErrorInfo } from "../api";
import type { KubernetesCluster } from "../cluster-types";

type VelauxCluster = {
  name?: string;
  alias?: string;
  description?: string;
  status?: string;
  kubeConfig?: string;
};

function velauxPath(clusterId: string, suffix = "") {
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/velaux/api/v1/clusters${suffix}`;
}

function parseVelauxList(data: unknown): { items: VelauxCluster[]; total: number } {
  const root = data && typeof data === "object" ? (data as Record<string, unknown>) : {};
  const nested = root.data && typeof root.data === "object" ? (root.data as Record<string, unknown>) : root;
  const items = Array.isArray(nested.clusters)
    ? (nested.clusters as VelauxCluster[])
    : Array.isArray(nested.items)
      ? (nested.items as VelauxCluster[])
      : [];
  const total = typeof nested.total === "number" ? nested.total : items.length;
  return { items, total };
}

export function ClusterAssociateDialog({ cluster }: { cluster: KubernetesCluster }) {
  const [error, setError] = useState<ApiErrorInfo | null>(null);
  const [items, setItems] = useState<VelauxCluster[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(10);
  const [loading, setLoading] = useState(true);
  const [joining, setJoining] = useState(false);
  const [form, setForm] = useState({ name: "", alias: "", description: "", kubeConfig: "" });
  const { ask, dialog: confirmDialog } = useK8sConfirm();

  const configured = Boolean(cluster.velaux_url?.trim());

  const load = useCallback(async () => {
    if (!configured) return;
    setError(null);
    setLoading(true);
    try {
      const data = await k8sFetch<unknown>(`${velauxPath(cluster.id)}?page=${page}&pageSize=${pageSize}&query=`);
      const parsed = parseVelauxList(data);
      setItems(parsed.items);
      setTotal(parsed.total);
    } catch (err) {
      setError(errorInfo(err, "加载 Velaux 集群失败"));
    } finally {
      setLoading(false);
    }
  }, [cluster.id, configured, page, pageSize]);

  useEffect(() => {
    void load();
  }, [load]);

  async function join() {
    if (!form.name.trim() || !form.kubeConfig.trim()) {
      setError({ code: "INVALID_REQUEST", message: "请填写名称和 KubeConfig" });
      return;
    }
    setError(null);
    try {
      await k8sFetch(velauxPath(cluster.id), {
        method: "POST",
        body: JSON.stringify({
          name: form.name.trim(),
          alias: form.alias.trim(),
          description: form.description.trim(),
          kubeConfig: form.kubeConfig,
        }),
      });
      setJoining(false);
      setForm({ name: "", alias: "", description: "", kubeConfig: "" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "关联集群失败"));
    }
  }

  async function remove(name: string) {
    const ok = await ask({
      title: "删除 Velaux 关联",
      description: `确定从 Velaux 删除集群「${name}」？`,
      confirmLabel: "删除",
      danger: true,
    });
    if (!ok) return;
    try {
      await k8sFetch(`${velauxPath(cluster.id)}/${encodeURIComponent(name)}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除关联失败"));
    }
  }

  if (!configured) {
    return (
      <K8sEmptyState
        variant="empty"
        title="未配置 Velaux"
        description="请先在「变更配置」中填写 Velaux 地址。"
        compact
      />
    );
  }

  return (
    <div className="stack-sm">
      <ApiErrorBanner error={error} />
      <div className="k8s-dialog-toolbar">
        <span className="text-xs text-muted">{loading ? "…" : `${total} 项`}</span>
        <div className="k8s-dialog-toolbar-actions">
          <button type="button" className="btn-secondary text-sm" disabled={loading} onClick={() => void load()}>
            ↻
          </button>
          <button type="button" className="btn text-sm" onClick={() => setJoining(true)}>
            注册
          </button>
        </div>
      </div>
      <div className="table-wrap k8s-dialog-table">
        <table className="data-table text-sm">
          <colgroup>
            <col />
            <col style={{ width: "4rem" }} />
            <col style={{ width: "25%" }} />
            <col style={{ width: "5rem" }} />
            <col style={{ width: "4rem" }} />
          </colgroup>
          <thead>
            <tr>
              <th>名称</th>
              <th className="col-compact">角色</th>
              <th>别名</th>
              <th className="col-compact">状态</th>
              <th className="col-actions">操作</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.name}>
                <td>{item.name}</td>
                <td className="text-muted">{item.name === "local" ? "管控" : "被控"}</td>
                <td className="text-muted">{item.alias || "—"}</td>
                <td className="text-muted">{item.status || "—"}</td>
                <td className="col-actions">
                  {item.name === "local" ? (
                    <span className="text-muted text-xs">—</span>
                  ) : (
                    <button type="button" className="btn-ghost text-sm" onClick={() => item.name && void remove(item.name)}>
                      删除
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {!loading && items.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <K8sEmptyState variant="empty" title="暂无关联" compact />
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      {total > 0 ? (
        <K8sPagination
          page={page}
          pageSize={pageSize}
          total={total}
          pageSizes={[10, 20, 50]}
          loading={loading}
          onPageChange={setPage}
          onPageSizeChange={(next) => {
            setPageSize(next);
            setPage(1);
          }}
        />
      ) : null}

      <Modal
        nested
        open={joining}
        title="注册集群"
        size="lg"
        panelClassName="k8s-cluster-form-modal"
        onClose={() => setJoining(false)}
        footer={
          <div className="modal-footer-actions">
            <button type="button" className="btn-secondary" onClick={() => setJoining(false)}>
              取消
            </button>
            <button type="button" className="btn" onClick={() => void join()}>
              注册
            </button>
          </div>
        }
      >
        <div className="k8s-cluster-form-section-body stack-sm">
          <label className="k8s-form-field">
            <span className="label">
              名称<span className="text-danger"> *</span>
            </span>
            <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </label>
          <label className="k8s-form-field">
            <span className="label">别名</span>
            <input className="input" value={form.alias} onChange={(e) => setForm({ ...form, alias: e.target.value })} />
          </label>
          <label className="k8s-form-field">
            <span className="label">描述</span>
            <input className="input" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
          </label>
          <label className="k8s-form-field">
            <span className="label">
              KubeConfig<span className="text-danger"> *</span>
            </span>
            <textarea
              className="input font-mono text-xs k8s-cluster-credential-input"
              rows={8}
              value={form.kubeConfig}
              onChange={(e) => setForm({ ...form, kubeConfig: e.target.value })}
            />
          </label>
        </div>
      </Modal>
      {confirmDialog}
    </div>
  );
}
