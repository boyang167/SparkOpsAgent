"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import { governance } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { DataResourceVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { MetricCard } from "@/components/ui/MetricCard";

interface PermissionDataPanelProps {
  workspaceId: string;
}

export function PermissionDataPanel({ workspaceId }: PermissionDataPanelProps) {
  const [resources, setResources] = useState<DataResourceVM[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setResources(await governance.listDataResources(workspaceId));
    } catch (err) {
      setError(err instanceof BffError ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  }, [workspaceId]);

  useEffect(() => {
    void load();
  }, [load]);

  return (
    <div className="stack-lg">
      <div className="menu-admin-toolbar-top">
        <p className="text-sm text-muted">
          查看系统已注册的数据资源。为角色分配数据范围请前往
          {" "}
          <Link href="/admin/roles" className="text-primary hover:underline">
            角色管理
          </Link>
          ，点击「数据权限」。
        </p>
        <button type="button" className="btn-secondary" onClick={() => void load()} disabled={loading}>
          刷新
        </button>
      </div>

      <div className="menu-admin-stats">
        <MetricCard
          label="数据资源"
          value={String(resources.length)}
          hint="可参与行级授权"
          tone="sky"
          icon={
            <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <ellipse cx="12" cy="5" rx="8" ry="3" />
              <path d="M4 5v6c0 1.7 3.6 3 8 3s8-1.3 8-3V5" />
              <path d="M4 11v6c0 1.7 3.6 3 8 3s8-1.3 8-3v-6" />
            </svg>
          }
        />
      </div>

      {error ? <InlineAlert tone="error">{error}</InlineAlert> : null}

      <div className="card menu-admin-main">
        {loading ? (
          <div className="menu-admin-skeleton">
            {Array.from({ length: 4 }).map((_, i) => (
              <div key={i} className="menu-admin-skeleton-row" style={{ width: `${90 - i * 5}%` }} />
            ))}
          </div>
        ) : resources.length === 0 ? (
          <div className="menu-admin-empty">暂无数据资源</div>
        ) : (
          <div className="table-wrap">
            <table className="data-table text-sm">
              <thead>
                <tr>
                  <th>资源键</th>
                  <th>名称</th>
                  <th>物理表</th>
                  <th>所有者字段</th>
                  <th>说明</th>
                </tr>
              </thead>
              <tbody>
                {resources.map((resource) => (
                  <tr key={resource.id}>
                    <td className="font-mono text-xs">{resource.resource_key}</td>
                    <td>{resource.title}</td>
                    <td className="font-mono text-xs">
                      {resource.schema_name}.{resource.table_name}
                    </td>
                    <td className="font-mono text-xs">{resource.owner_column}</td>
                    <td className="text-muted">{resource.description || "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>
    </div>
  );
}
