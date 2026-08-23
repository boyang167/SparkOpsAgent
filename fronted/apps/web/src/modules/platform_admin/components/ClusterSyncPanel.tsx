"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { SyncJobVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";

interface ClusterSyncPanelProps {
  workspaceId: string;
  variant?: "platform" | "default";
}

export function ClusterSyncPanel({ workspaceId, variant = "default" }: ClusterSyncPanelProps) {
  const isPlatform = variant === "platform";
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

  const createBtn = (
    <button type="button" className="btn platform-add-btn" onClick={() => setOpen(true)}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
        <path d="M12 5v14M5 12h14" strokeLinecap="round" />
      </svg>
      新建同步任务
    </button>
  );

  const body = (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>加载中…</div>
      ) : items.length === 0 ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>暂无同步任务</div>
      ) : (
        <div className={isPlatform ? "platform-table-wrap" : "table-wrap"}>
          <table className={isPlatform ? "platform-table" : "data-table text-sm"}>
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
      {!loading && items.length > 0 && isPlatform ? (
        <div className="platform-table-footer">
          <span className="text-sm text-muted">共 {items.length} 个同步任务</span>
        </div>
      ) : null}
    </>
  );

  return (
    <div className={isPlatform ? "platform-orgs-page" : "stack-md"}>
      {isPlatform ? (
        <section className="platform-table-card platform-orgs-card">
          <PlatformPanelHead
            title="资源同步"
            description="跨集群资源同步任务管理。"
            action={createBtn}
          />
          <div className="platform-orgs-card-body">{body}</div>
        </section>
      ) : (
        <>
          <div className="flex justify-end">{createBtn}</div>
          {body}
        </>
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
