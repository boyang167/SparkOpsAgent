"use client";

import type { MetricPoint } from "./project-monitor-utils";

interface ProjectMonitorMetricChartProps {
  title: string;
  subtitle?: string;
  points: MetricPoint[];
  tone: "cpu" | "memory";
  formatValue: (value: number) => string;
}

export function ProjectMonitorMetricChart({
  title,
  subtitle,
  points,
  tone,
  formatValue,
}: ProjectMonitorMetricChartProps) {
  const width = 640;
  const height = 168;
  const paddingX = 28;
  const paddingY = 24;

  if (points.length === 0) {
    return (
      <section className="project-monitor-chart">
        <header className="project-monitor-chart-head">
          <div>
            <h4>{title}</h4>
            {subtitle ? <p>{subtitle}</p> : null}
          </div>
        </header>
        <div className="project-monitor-chart-empty">暂无趋势数据（请确认集群已配置 Prometheus）</div>
      </section>
    );
  }

  const values = points.map((point) => point.value);
  const min = Math.min(...values);
  const max = Math.max(...values);
  const span = max - min || 1;
  const polyline = points
    .map((point, index) => {
      const x =
        paddingX + (index / Math.max(points.length - 1, 1)) * (width - paddingX * 2);
      const y =
        height - paddingY - ((point.value - min) / span) * (height - paddingY * 2);
      return `${x},${y}`;
    })
    .join(" ");

  const latest = points[points.length - 1]?.value ?? 0;
  const peak = Math.max(...values);

  return (
    <section className="project-monitor-chart">
      <header className="project-monitor-chart-head">
        <div>
          <h4>{title}</h4>
          {subtitle ? <p>{subtitle}</p> : null}
        </div>
        <div className="project-monitor-chart-stats">
          <span>
            当前 <strong>{formatValue(latest)}</strong>
          </span>
          <span>
            峰值 <strong>{formatValue(peak)}</strong>
          </span>
        </div>
      </header>
      <svg
        viewBox={`0 0 ${width} ${height}`}
        className={`project-monitor-chart-svg is-${tone}`}
        role="img"
        aria-label={`${title}趋势图`}
      >
        <defs>
          <linearGradient id={`project-monitor-fill-${tone}`} x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="currentColor" stopOpacity="0.18" />
            <stop offset="100%" stopColor="currentColor" stopOpacity="0.02" />
          </linearGradient>
        </defs>
        <polyline
          className="project-monitor-chart-line"
          points={polyline}
          fill="none"
          strokeWidth="2.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </section>
  );
}
