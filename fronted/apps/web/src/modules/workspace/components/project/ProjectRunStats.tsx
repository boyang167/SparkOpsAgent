"use client";

import type { BuildVM } from "@sparkops/contracts/delivery";
import type { DeploymentRunVM } from "@sparkops/contracts/delivery";
import { buildStatusLabel, buildStatusTone } from "./build-summary";
import { deploymentRunStatusLabel, deploymentStatusTone } from "./deployment-console-utils";
import { formatCompactDateTime } from "./project-utils";

export interface RunStatSlice<T> {
  total: number | null;
  latest: T | null;
  loading: boolean;
  error: string | null;
}

interface ProjectRunStatsProps {
  deployment: RunStatSlice<DeploymentRunVM>;
  build: RunStatSlice<BuildVM>;
}

function formatMetricValue(slice: RunStatSlice<unknown>): string {
  if (slice.loading) return "—";
  if (slice.error) return "—";
  return String(slice.total ?? 0);
}

function metricState(slice: RunStatSlice<unknown>): "loading" | "error" | "ready" {
  if (slice.loading) return "loading";
  if (slice.error) return "error";
  return "ready";
}

function StatusBadge({ label, tone }: { label: string; tone: string }) {
  return (
    <span className={`project-status-badge is-${tone}`}>
      <span className="project-status-badge-dot" aria-hidden />
      {label}
    </span>
  );
}

function MetricBlock({
  label,
  value,
  muted,
  title,
}: {
  label: string;
  value: string;
  muted: boolean;
  title?: string;
}) {
  return (
    <div className="project-run-metric">
      <span className="project-run-metric-label">{label}</span>
      <strong className={`project-run-metric-value${muted ? " is-muted" : ""}`} title={title}>
        {value}
      </strong>
    </div>
  );
}

function ActivityRow({
  label,
  loading,
  error,
  empty,
  statusLabel,
  statusTone,
  time,
}: {
  label: string;
  loading: boolean;
  error: string | null;
  empty: boolean;
  statusLabel?: string;
  statusTone?: string;
  time?: string;
}) {
  return (
    <div className="project-run-activity">
      <span className="project-run-activity-label">{label}</span>
      <div className="project-run-activity-value">
        {loading ? (
          <span className="project-run-activity-muted">加载中…</span>
        ) : error ? (
          <span className="project-run-activity-muted">不可用</span>
        ) : empty ? (
          <span className="project-run-activity-muted">暂无记录</span>
        ) : (
          <>
            {statusLabel && statusTone ? (
              <StatusBadge label={statusLabel} tone={statusTone} />
            ) : null}
            {time ? (
              <time className="project-run-activity-time" dateTime={time} title={time}>
                {formatCompactDateTime(time)}
              </time>
            ) : null}
          </>
        )}
      </div>
    </div>
  );
}

export function ProjectRunStats({ deployment, build }: ProjectRunStatsProps) {
  const deploymentMetric = metricState(deployment);
  const buildMetric = metricState(build);

  return (
    <section className="project-overview-panel project-run-panel" aria-label="运行概况">
      <header className="project-overview-panel-head">
        <h4 className="project-overview-panel-title">运行概况</h4>
      </header>
      <div className="project-run-metrics">
        <MetricBlock
          label="部署"
          value={formatMetricValue(deployment)}
          muted={deploymentMetric !== "ready"}
          title={deployment.error ?? undefined}
        />
        <MetricBlock
          label="构建"
          value={formatMetricValue(build)}
          muted={buildMetric !== "ready"}
          title={build.error ?? undefined}
        />
      </div>
      <div className="project-run-activities">
        <ActivityRow
          label="最近部署"
          loading={deployment.loading}
          error={deployment.error}
          empty={!deployment.latest}
          statusLabel={
            deployment.latest ? deploymentRunStatusLabel(deployment.latest.status) : undefined
          }
          statusTone={
            deployment.latest ? deploymentStatusTone(deployment.latest.status) : undefined
          }
          time={deployment.latest?.created_at}
        />
        <ActivityRow
          label="最近构建"
          loading={build.loading}
          error={build.error}
          empty={!build.latest}
          statusLabel={build.latest ? buildStatusLabel(build.latest.status) : undefined}
          statusTone={build.latest ? buildStatusTone(build.latest.status) : undefined}
          time={build.latest?.started_at}
        />
      </div>
    </section>
  );
}
