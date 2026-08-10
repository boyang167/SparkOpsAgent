"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { DeploymentStatVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface DeploymentStatsPanelProps {
  workspaceId: string;
}

export function DeploymentStatsPanel({ workspaceId }: DeploymentStatsPanelProps) {
  const [items, setItems] = useState<DeploymentStatVM[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setItems(await platform.listDeploymentStats(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  if (loading) return <div className="empty-state">加载中…</div>;

  return (
    <div className="stack-md">
      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}
      {items.length === 0 ? (
        <div className="empty-state">暂无统计数据</div>
      ) : (
        <div className="table-wrap">
          <table className="data-table text-sm">
            <thead>
              <tr>
                <th>日期</th>
                <th>项目</th>
                <th>部署次数</th>
              </tr>
            </thead>
            <tbody>
              {items.map((row, index) => (
                <tr key={`${row.date}-${index}`}>
                  <td>{row.date}</td>
                  <td>{row.project_name ?? "全部"}</td>
                  <td>{row.count}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
