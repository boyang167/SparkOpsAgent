"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { notifyEntitlementsUpdated } from "@/components/layout/shell-context";
import { useConfirmDialog } from "@/hooks/useConfirmDialog";

interface WorkspaceClusterPanelProps {
  workspaceId: string;
}

function moveItem<T>(items: T[], from: number, to: number): T[] {
  if (from === to || from < 0 || to < 0 || from >= items.length || to >= items.length) {
    return items;
  }
  const next = [...items];
  const [item] = next.splice(from, 1);
  next.splice(to, 0, item);
  return next;
}

function withRank(items: AuthorizedClusterVM[]): AuthorizedClusterVM[] {
  return items.map((item, index) => ({
    ...item,
    priority_id: index,
    is_build_env: index === 0,
  }));
}

export function WorkspaceClusterPanel({ workspaceId }: WorkspaceClusterPanelProps) {
  const [items, setItems] = useState<AuthorizedClusterVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const { confirm, ConfirmDialog } = useConfirmDialog();

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(withRank(await platform.listAuthorizedWorkspaceClusters(workspaceId)));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载集群失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function persistOrder(nextItems: AuthorizedClusterVM[]) {
    setSaving(true);
    setError(null);
    try {
      await platform.reorderAuthorizedWorkspaceClusters(
        workspaceId,
        nextItems.map((item) => item.id),
      );
      setItems(withRank(nextItems));
      notifyEntitlementsUpdated();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "更新集群排序失败");
      await load();
    } finally {
      setSaving(false);
    }
  }

  async function onMove(index: number, direction: "up" | "down") {
    const target = direction === "up" ? index - 1 : index + 1;
    const nextItems = moveItem(items, index, target);
    await persistOrder(nextItems);
  }

  async function onRevoke(clusterId: string) {
    const ok = await confirm({ title: "确定从当前工作空间撤销该集群授权？", danger: true });
    if (!ok) return;
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
    <>
    <div className="stack-md">
      <p className="text-sm text-muted">
        拖拽排序暂未开放，请使用上移/下移调整顺序。排在最前的集群为<strong>构建集群</strong>
        ，发布时在此构建镜像并部署；其余集群用于后续跨集群同步配置。
      </p>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">当前工作空间暂无授权集群。</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm workspace-cluster-table">
            <thead>
              <tr>
                <th>排序</th>
                <th>名称</th>
                <th>角色</th>
                <th>API Server</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row, index) => (
                <tr key={row.id}>
                  <td>
                    <div className="workspace-cluster-sort-actions">
                      <button
                        type="button"
                        className="btn-ghost text-xs"
                        disabled={saving || index === 0}
                        onClick={() => void onMove(index, "up")}
                        aria-label={`上移 ${row.cluster_name}`}
                      >
                        ↑
                      </button>
                      <button
                        type="button"
                        className="btn-ghost text-xs"
                        disabled={saving || index === items.length - 1}
                        onClick={() => void onMove(index, "down")}
                        aria-label={`下移 ${row.cluster_name}`}
                      >
                        ↓
                      </button>
                    </div>
                  </td>
                  <td>{row.cluster_name}</td>
                  <td>
                    {row.is_build_env ? (
                      <span className="project-tag project-tag-green">构建集群</span>
                    ) : (
                      <span className="project-tag project-tag-blue">同步集群</span>
                    )}
                  </td>
                  <td className="font-mono text-xs">{row.server || "-"}</td>
                  <td>
                    <button
                      type="button"
                      className="btn-secondary"
                      disabled={saving}
                      onClick={() => void onRevoke(row.id)}
                    >
                      撤销授权
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
    <ConfirmDialog />
    </>
  );
}
