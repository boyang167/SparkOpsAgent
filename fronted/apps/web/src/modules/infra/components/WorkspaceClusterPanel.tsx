"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";

interface WorkspaceClusterPanelProps {
  workspaceId: string;
}

export function WorkspaceClusterPanel({ workspaceId }: WorkspaceClusterPanelProps) {
  const [items, setItems] = useState<AuthorizedClusterVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [open, setOpen] = useState(false);
  const [clusterName, setClusterName] = useState("");
  const [server, setServer] = useState("");
  const [submitting, setSubmitting] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await platform.listAuthorizedWorkspaceClusters(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载集群失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function onCreate(event: React.FormEvent) {
    event.preventDefault();
    setSubmitting(true);
    setError(null);
    try {
      await platform.registerWorkspaceCluster(workspaceId, {
        cluster_name: clusterName.trim(),
        server: server.trim() || undefined,
      });
      setOpen(false);
      setClusterName("");
      setServer("");
      notifyEntitlementsUpdated();
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "注册集群失败");
    } finally {
      setSubmitting(false);
    }
  }

  async function onRevoke(clusterId: string) {
    if (!window.confirm("确定从当前工作空间撤销该集群授权？")) return;
    setError(null);
    try {
      await platform.revokeAuthorizedWorkspaceCluster(workspaceId, clusterId);
      notifyEntitlementsUpdated();
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "撤销授权失败");
    }
  }

  return (
    <div className="stack-md">
      <div className="flex justify-end">
        <button type="button" className="btn platform-add-btn" onClick={() => setOpen(true)}>
          注册集群
        </button>
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">当前工作空间暂无授权集群，点击「注册集群」接入 Kubernetes。</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>名称</th>
                <th>API Server</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row) => (
                <tr key={row.id}>
                  <td>{row.cluster_name}</td>
                  <td className="font-mono text-xs">{row.server || "-"}</td>
                  <td>
                    <button type="button" className="btn-secondary" onClick={() => void onRevoke(row.id)}>
                      撤销授权
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <Modal open={open} onClose={() => setOpen(false)} title="注册 K8s 集群">
        <form onSubmit={(event) => void onCreate(event)} className="stack-md">
          <input
            className="input"
            placeholder="集群名称"
            value={clusterName}
            onChange={(event) => setClusterName(event.target.value)}
            required
          />
          <input
            className="input"
            placeholder="API Server URL，例如 https://192.168.1.10:6443"
            value={server}
            onChange={(event) => setServer(event.target.value)}
          />
          <p className="text-sm text-muted">
            注册后会自动授权给当前工作空间。kubeconfig 请在「密钥配置」中以 Kubernetes 密钥保存。
          </p>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setOpen(false)}>
              取消
            </button>
            <button type="submit" className="btn" disabled={submitting}>
              {submitting ? "保存中…" : "保存"}
            </button>
          </div>
        </form>
      </Modal>
    </div>
  );
}
