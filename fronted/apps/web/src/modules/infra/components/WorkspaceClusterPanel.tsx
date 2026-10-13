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
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : items.length === 0 ? (
        <div className="empty-state">当前工作空间暂无授权集群。</div>
      ) : (
        <ul className="runtime-cluster-list" role="list">
          {items.map((row, index) => (
            <li key={row.id} className={`runtime-cluster-card${row.is_build_env ? " is-primary" : ""}`}>
              <div className="runtime-cluster-leading">
                <span className="runtime-cluster-rank">{index + 1}</span>
                <div className="runtime-cluster-sort">
                  <button
                    type="button"
                    className="runtime-sort-btn"
                    disabled={saving || index === 0}
                    onClick={() => void onMove(index, "up")}
                    aria-label={`上移 ${row.cluster_name}`}
                  >
                    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                      <path d="M4 10l4-4 4 4" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                  <button
                    type="button"
                    className="runtime-sort-btn"
                    disabled={saving || index === items.length - 1}
                    onClick={() => void onMove(index, "down")}
                    aria-label={`下移 ${row.cluster_name}`}
                  >
                    <svg viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" aria-hidden="true">
                      <path d="M4 6l4 4 4-4" strokeLinecap="round" strokeLinejoin="round" />
                    </svg>
                  </button>
                </div>
              </div>
              <div className="runtime-cluster-body">
                <div className="runtime-cluster-name-row">
                  <strong className="runtime-cluster-name">{row.cluster_name}</strong>
                  {row.is_build_env ? (
                    <span className="runtime-role-tag is-build">构建集群</span>
                  ) : (
                    <span className="runtime-role-tag is-sync">同步集群</span>
                  )}
                </div>
                {row.server ? (
                  <span className="runtime-cluster-server" title={row.server}>{row.server}</span>
                ) : null}
              </div>
              <div className="runtime-cluster-trailing">
                <button
                  type="button"
                  className="runtime-revoke-btn"
                  disabled={saving}
                  onClick={() => void onRevoke(row.id)}
                >
                  撤销授权
                </button>
              </div>
            </li>
          ))}
        </ul>
      )}
    </div>
    <ConfirmDialog />
    </>
  );
}
