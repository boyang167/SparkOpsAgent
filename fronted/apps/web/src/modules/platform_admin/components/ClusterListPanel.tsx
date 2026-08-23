"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { PlatformPanelHead } from "@/components/ui/PlatformPanelHead";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";

interface ClusterListPanelProps {
  variant?: "platform" | "default";
}

export function ClusterListPanel({ variant = "default" }: ClusterListPanelProps) {
  const isPlatform = variant === "platform";
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
      notifyEntitlementsUpdated();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "创建失败");
    }
  }

  const registerBtn = (
    <button type="button" className="btn platform-add-btn" onClick={() => setOpen(true)}>
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden>
        <path d="M12 5v14M5 12h14" strokeLinecap="round" />
      </svg>
      注册集群
    </button>
  );

  const body = (
    <>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>加载中…</div>
      ) : items.length === 0 ? (
        <div className={isPlatform ? "platform-table-empty" : "empty-state"}>
          暂无集群，点击「注册集群」添加
        </div>
      ) : (
        <div className={isPlatform ? "platform-table-wrap" : "table-wrap"}>
          <table className={isPlatform ? "platform-table" : "data-table text-sm"}>
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
      {!loading && items.length > 0 && isPlatform ? (
        <div className="platform-table-footer">
          <span className="text-sm text-muted">共 {items.length} 个集群</span>
        </div>
      ) : null}
    </>
  );

  return (
    <div className={isPlatform ? "platform-orgs-page" : "stack-md"}>
      {isPlatform ? (
        <section className="platform-table-card platform-orgs-card">
          <PlatformPanelHead
            title="集群管理"
            description="多集群接入与 Kubeconfig 注册。"
            action={registerBtn}
          />
          <div className="platform-orgs-card-body">{body}</div>
        </section>
      ) : (
        <>
          <div className="flex justify-end">{registerBtn}</div>
          {body}
        </>
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
