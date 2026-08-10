"use client";

import { useCallback, useEffect, useState } from "react";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { RegisteredAPI } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { Modal } from "@/components/ui/Modal";

interface PermissionApisPanelProps {
  workspaceId: string;
  canSync: boolean;
}

type SyncRow = RegisteredAPI & { api_group?: string; description?: string };

export function PermissionApisPanel({ workspaceId, canSync }: PermissionApisPanelProps) {
  const [apis, setApis] = useState<RegisteredAPI[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [syncOpen, setSyncOpen] = useState(false);
  const [syncLoading, setSyncLoading] = useState(false);
  const [newApis, setNewApis] = useState<SyncRow[]>([]);
  const [deleteApis, setDeleteApis] = useState<SyncRow[]>([]);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setApis(await governance.listWorkspaceApis(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  async function openSync() {
    setSyncLoading(true);
    setError(null);
    try {
      const diff = await governance.previewApiSync();
      setNewApis(
        (diff.new as SyncRow[]).map((item) => ({
          ...item,
          id: item.id ?? "",
          path: item.path ?? "",
          method: item.method ?? "GET",
          api_group: item.api_group ?? "default",
          description: item.description ?? "",
        })),
      );
      setDeleteApis(diff.delete as SyncRow[]);
      setSyncOpen(true);
    } catch (err) {
      setError(err instanceof BffError ? err.message : "同步预览失败（需 Super Admin）");
    } finally {
      setSyncLoading(false);
    }
  }

  async function confirmSync() {
    setSyncLoading(true);
    try {
      await governance.confirmApiSync({ new: newApis, delete: deleteApis });
      setSyncOpen(false);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "同步失败");
    } finally {
      setSyncLoading(false);
    }
  }

  return (
    <div className="section stack-md">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="section-title">API 管理</h2>
          <p className="text-muted mt-1">从后端路由同步 API 注册表（参照 gin-vue-admin）</p>
        </div>
        {canSync ? (
          <button type="button" className="btn" disabled={syncLoading} onClick={() => void openSync()}>
            {syncLoading ? "扫描中…" : "同步 API"}
          </button>
        ) : null}
      </div>
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {loading ? (
        <div className="empty-state">加载中…</div>
      ) : apis.length === 0 ? (
        <div className="empty-state">暂无 API 记录，Super Admin 可点击「同步 API」扫描路由</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>分组</th>
                <th>Method</th>
                <th>路径</th>
                <th>描述</th>
              </tr>
            </thead>
            <tbody>
              {apis.map((row) => (
                <tr key={`${row.method}:${row.path}`}>
                  <td>{row.api_group || "-"}</td>
                  <td>
                    <span className="badge">{row.method}</span>
                  </td>
                  <td className="font-mono text-xs">{row.path}</td>
                  <td>{row.description || "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      <Modal open={syncOpen} onClose={() => setSyncOpen(false)} title="API 同步预览">
        <div className="stack-md max-h-[70vh] overflow-y-auto">
          <div>
            <h3 className="font-medium">新增 ({newApis.length})</h3>
            {newApis.length === 0 ? (
              <p className="text-sm text-muted">无新增 API</p>
            ) : (
              newApis.map((row, index) => (
                <div key={`${row.method}:${row.path}`} className="grid gap-2 sm:grid-cols-3 mt-2">
                  <span className="text-xs font-mono col-span-3">
                    {row.method} {row.path}
                  </span>
                  <input
                    className="input"
                    placeholder="分组"
                    value={row.api_group ?? ""}
                    onChange={(e) =>
                      setNewApis((prev) =>
                        prev.map((item, i) => (i === index ? { ...item, api_group: e.target.value } : item)),
                      )
                    }
                  />
                  <input
                    className="input sm:col-span-2"
                    placeholder="描述"
                    value={row.description ?? ""}
                    onChange={(e) =>
                      setNewApis((prev) =>
                        prev.map((item, i) => (i === index ? { ...item, description: e.target.value } : item)),
                      )
                    }
                  />
                </div>
              ))
            )}
          </div>
          <div>
            <h3 className="font-medium">删除 ({deleteApis.length})</h3>
            {deleteApis.length === 0 ? (
              <p className="text-sm text-muted">无待删除 API</p>
            ) : (
              <ul className="text-xs font-mono text-muted space-y-1">
                {deleteApis.map((row) => (
                  <li key={`${row.method}:${row.path}`}>
                    {row.method} {row.path}
                  </li>
                ))}
              </ul>
            )}
          </div>
          <div className="flex justify-end gap-2">
            <button type="button" className="btn-secondary" onClick={() => setSyncOpen(false)}>
              取消
            </button>
            <button type="button" className="btn" disabled={syncLoading} onClick={() => void confirmSync()}>
              确认同步
            </button>
          </div>
        </div>
      </Modal>
    </div>
  );
}
