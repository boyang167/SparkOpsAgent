"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

export function ClusterListPanel() {
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [clusterName, setClusterName] = useState("");
  const [server, setServer] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await platform.listClusters());
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败（需 Super Admin）");
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(e: React.FormEvent) {
    e.preventDefault();
    try {
      await platform.createCluster({ cluster_name: clusterName, server });
      setOpen(false);
      setClusterName("");
      setServer("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    }
  }

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn" onClick={() => setOpen(true)}>
          注册集群
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无集群，点击「注册集群」添加</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>API Server</th>
                <th>构建环境</th>
                <th>优先级</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={String(row.id)}>
                  <td>{String(row.cluster_name ?? row.name ?? "-")}</td>
                  <td className="font-mono text-xs">{String(row.server ?? row.api_server ?? "-")}</td>
                  <td>{row.is_build_env ? "是" : "否"}</td>
                  <td>{String(row.priority_id ?? "-")}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="注册 K8s 集群">
        <form onSubmit={onCreate} className="stack-md">
          <input className="input" placeholder="集群名称" value={clusterName} onChange={(e) => setClusterName(e.target.value)} required />
          <input className="input" placeholder="API Server URL" value={server} onChange={(e) => setServer(e.target.value)} />
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>取消</button>
            <button type="submit" className="btn">保存</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
