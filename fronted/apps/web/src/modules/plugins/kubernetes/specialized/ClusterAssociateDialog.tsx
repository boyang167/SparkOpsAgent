"use client";

import { useCallback, useEffect, useState } from "react";
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
  const [joining, setJoining] = useState(false);
  const [form, setForm] = useState({ name: "", alias: "", description: "", kubeConfig: "" });

  const configured = Boolean(cluster.velaux_url?.trim());

  const load = useCallback(async () => {
    if (!configured) return;
    setError(null);
    try {
      const data = await k8sFetch<unknown>(`${velauxPath(cluster.id)}?page=${page}&pageSize=10&query=`);
      const parsed = parseVelauxList(data);
      setItems(parsed.items);
      setTotal(parsed.total);
    } catch (err) {
      setError(errorInfo(err, "加载 Velaux 集群失败。请确认已在「变更」中填写 Velaux 地址，并已安装 KubeVela/Velaux。"));
    }
  }, [cluster.id, configured, page]);

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
    if (!window.confirm(`确定从 Velaux 删除集群「${name}」？`)) return;
    try {
      await k8sFetch(`${velauxPath(cluster.id)}/${encodeURIComponent(name)}`, { method: "DELETE" });
      await load();
    } catch (err) {
      setError(errorInfo(err, "删除关联失败"));
    }
  }

  if (!configured) {
    return <div className="alert alert-info">未配置 Velaux 地址。请先在「变更」中填写 Velaux 接口地址后再关联多集群。</div>;
  }

  return (
    <div className="stack-md">
      <p className="text-sm text-muted">请在目标 Kubernetes 中安装 KubeVela 控制器及 Velaux 插件（建议 KubeVela ≥ v1.9）。</p>
      <ApiErrorBanner error={error} />
      <div className="flex flex-wrap gap-2">
        <button type="button" className="btn" onClick={() => setJoining(true)}>
          添加
        </button>
        <button type="button" className="btn-secondary" onClick={() => void load()}>
          刷新
        </button>
      </div>
      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>名称</th>
              <th>角色</th>
              <th>别名</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {items.map((item) => (
              <tr key={item.name}>
                <td>{item.name}</td>
                <td>{item.name === "local" ? "管控节点" : "被控节点"}</td>
                <td>{item.alias || "-"}</td>
                <td>{item.status || "-"}</td>
                <td>
                  {item.name === "local" ? (
                    "-"
                  ) : (
                    <button type="button" className="k8s-action-link" onClick={() => item.name && void remove(item.name)}>
                      删除
                    </button>
                  )}
                </td>
              </tr>
            ))}
            {items.length === 0 ? (
              <tr>
                <td colSpan={5}>
                  <div className="empty-state">暂无 Velaux 关联集群</div>
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      <div className="text-sm text-muted">共 {total} 条</div>

      <Modal
        open={joining}
        title="注册集群"
        size="lg"
        onClose={() => setJoining(false)}
        footer={
          <>
            <button type="button" className="btn-secondary" onClick={() => setJoining(false)}>
              取消
            </button>
            <button type="button" className="btn" onClick={() => void join()}>
              确定
            </button>
          </>
        }
      >
        <div className="stack-md">
          <label>
            <span className="label">名称</span>
            <input className="input" value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </label>
          <label>
            <span className="label">别名</span>
            <input className="input" value={form.alias} onChange={(e) => setForm({ ...form, alias: e.target.value })} />
          </label>
          <label>
            <span className="label">描述</span>
            <input className="input" value={form.description} onChange={(e) => setForm({ ...form, description: e.target.value })} />
          </label>
          <label>
            <span className="label">KubeConfig</span>
            <textarea
              className="input font-mono text-xs"
              rows={10}
              value={form.kubeConfig}
              onChange={(e) => setForm({ ...form, kubeConfig: e.target.value })}
            />
          </label>
        </div>
      </Modal>
    </div>
  );
}
