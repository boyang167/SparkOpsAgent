"use client";

import { useCallback, useEffect, useState } from "react";
import { platform } from "@sparkops/web-sdk";
import { BffError } from "@sparkops/contracts";
import type { MonitorOverviewVM } from "@sparkops/contracts/platform";
import { InlineAlert } from "@/components/ui/InlineAlert";

interface MonitorOverviewPanelProps {
  workspaceId: string;
}

function MonitorLinkCard({
  title,
  description,
  url,
  emptyHint,
}: {
  title: string;
  description: string;
  url?: string;
  emptyHint: string;
}) {
  return (
    <article className="monitor-link-card">
      <h3 className="monitor-link-card-title">{title}</h3>
      <p className="monitor-link-card-desc">{description}</p>
      {url ? (
        <a className="monitor-link-card-url" href={url} target="_blank" rel="noreferrer">
          {url}
        </a>
      ) : (
        <p className="monitor-link-card-desc">{emptyHint}</p>
      )}
    </article>
  );
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
      <div className="monitor-link-grid">
        <MonitorLinkCard
          title="Grafana"
          description="可视化仪表盘与告警面板"
          url={data?.grafana_url}
          emptyHint="未配置 Grafana 地址，可在系统配置中设置。"
        />
        <MonitorLinkCard
          title="Prometheus"
          description="指标采集与 PromQL 查询"
          url={data?.prometheus_url}
          emptyHint="未配置 Prometheus 地址，可在系统配置中设置 prometheus_url。"
        />
      </div>
    </div>
  );
}
