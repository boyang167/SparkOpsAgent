"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SyncJobVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface ClusterSyncPanelProps {
  workspaceId: string;
}

export function ClusterSyncPanel({ workspaceId }: ClusterSyncPanelProps) {
  const [items, setItems] = useState<SyncJobVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const [sourceCluster, setSourceCluster] = useState("");
  const [targetCluster, setTargetCluster] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await platform.listSyncJobs());
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
      await platform.createSyncJob({
        name,
        source_cluster: sourceCluster,
        target_cluster: targetCluster,
        sync_resources: "all",
      });
      setOpen(false);
      setName("");
      setSourceCluster("");
      setTargetCluster("");
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建同步任务失败");
    }
  }

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn" onClick={() => setOpen(true)}>新建同步任务</button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">暂无同步任务</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>源集群</th>
                <th>目标集群</th>
                <th>状态</th>
                <th>结果</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.id}>
                  <td>{row.name}</td>
                  <td>{row.source_cluster ?? "-"}</td>
                  <td>{row.target_cluster ?? "-"}</td>
                  <td><span className="badge">{row.status ?? "-"}</span></td>
                  <td className="text-xs text-muted">{row.result ?? "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="跨集群资源同步">
        <form onSubmit={onCreate} className="stack-md">
          <input className="input" placeholder="任务名称" value={name} onChange={(e) => setName(e.target.value)} required />
          <input className="input" placeholder="源集群" value={sourceCluster} onChange={(e) => setSourceCluster(e.target.value)} />
          <input className="input" placeholder="目标集群" value={targetCluster} onChange={(e) => setTargetCluster(e.target.value)} />
          <p className="text-xs text-muted">工作区上下文：{workspaceId.slice(0, 8)}…</p>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>取消</button>
            <button type="submit" className="btn">提交</button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
