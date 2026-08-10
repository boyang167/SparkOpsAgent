"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { MonitorOverviewVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface MonitorOverviewPanelProps {
  workspaceId: string;
}

export function MonitorOverviewPanel({ workspaceId }: MonitorOverviewPanelProps) {
  const [data, setData] = useState<MonitorOverviewVM | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      setData(await platform.getMonitorOverview(workspaceId));
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
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="rounded-lg border border-[var(--border)] p-4">
          <h3 className="font-medium">Grafana</h3>
          {data?.grafana_url ? (
            <a href={data.grafana_url} target="_blank" rel="noreferrer" className="text-sm text-primary hover:underline">
              {data.grafana_url}
            </a>
          ) : (
            <p className="text-sm text-muted mt-2">未配置。Super Admin 可在「集群 / 密钥配置」中设置 grafana_url。</p>
          )}
        </div>
        <div className="rounded-lg border border-[var(--border)] p-4">
          <h3 className="font-medium">Prometheus</h3>
          {data?.prometheus_url ? (
            <a href={data.prometheus_url} target="_blank" rel="noreferrer" className="text-sm text-primary hover:underline">
              {data.prometheus_url}
            </a>
          ) : (
            <p className="text-sm text-muted mt-2">未配置。可在系统配置中设置 prometheus_url。</p>
          )}
        </div>
      </div>
    </div>
  );
}
