"use client";

import Link from "next/link";
import { useCallback, useEffect, useMemo, useState } from "react";
import { product } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { WorkspaceProductWorkflowVM } from "@sparkops/contracts/product";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface WorkspaceWorkflowsPanelProps {
  workspaceId: string;
}

function statusTone(status: string): string {
  if (status === "active" || status === "success") return "text-green-600";
  if (status === "failed") return "text-red-600";
  if (status === "running") return "text-blue-600";
  return "text-muted-foreground";
}

export function WorkspaceWorkflowsPanel({ workspaceId }: WorkspaceWorkflowsPanelProps) {
  const [items, setItems] = useState<WorkspaceProductWorkflowVM[]>([]);
  const [query, setQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyId, setBusyId] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await product.listWorkspaceWorkflows(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载工作流失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return items.filter((item) => {
      if (statusFilter && item.status !== statusFilter) return false;
      if (!q) return true;
      return (
        item.name.toLowerCase().includes(q) ||
        item.key.toLowerCase().includes(q) ||
        (item.product_name ?? "").toLowerCase().includes(q)
      );
    });
  }, [items, query, statusFilter]);

  async function onExecute(wf: WorkspaceProductWorkflowVM) {
    setBusyId(wf.id);
    setError(null);
    try {
      await product.startProductWorkflowRun(workspaceId, wf.product_id, wf.id);
      await load();
    } catch (err) {
      setError(err instanceof BffError ? err.message : "执行失败");
    } finally {
      setBusyId(null);
    }
  }

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div className="flex flex-wrap gap-2 items-end">
        <label className="stack-xs text-sm">
          搜索
          <input
            className="input"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="名称 / 标识 / 产品"
          />
        </label>
        <label className="stack-xs text-sm">
          状态
          <select className="input" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="">全部</option>
            <option value="active">active</option>
            <option value="draft">draft</option>
          </select>
        </label>
        <button type="button" className="btn-secondary text-sm" onClick={() => void load()}>
          刷新
        </button>
      </div>

      <div className="table-wrap">
        <table className="data-table text-sm">
          <thead>
            <tr>
              <th>工作流</th>
              <th>产品</th>
              <th>阶段</th>
              <th>状态</th>
              <th>操作</th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 ? (
              <tr>
                <td colSpan={5} className="text-muted-foreground">
                  暂无产品工作流实例
                </td>
              </tr>
            ) : (
              filtered.map((wf) => (
                <tr key={wf.id}>
                  <td>
                    <div className="font-medium">{wf.name}</div>
                    <div className="text-xs text-muted-foreground">{wf.key}</div>
                  </td>
                  <td>
                    <Link href={`/products/${wf.product_id}`} className="text-primary hover:underline">
                      {wf.product_name || wf.product_id.slice(0, 8)}
                    </Link>
                  </td>
                  <td>{wf.stage_count}</td>
                  <td className={statusTone(wf.status)}>{wf.status}</td>
                  <td>
                    <div className="flex flex-wrap gap-1">
                      <button
                        type="button"
                        className="btn-primary text-xs"
                        disabled={busyId === wf.id}
                        onClick={() => void onExecute(wf)}
                      >
                        执行
                      </button>
                      <Link
                        href={`/products/${wf.product_id}`}
                        className="btn-secondary text-xs inline-flex items-center"
                      >
                        详情
                      </Link>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}
