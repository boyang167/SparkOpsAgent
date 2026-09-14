"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { BffError } from "@sparkops/contracts";
import type { PodVM, ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";
import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { cluster } from "@sparkops/web-sdk";
import { K8sEmptyState, K8sLoadingState } from "@/components/k8s";
import { InlineAlert } from "@/components/ui/InlineAlert";
import { formatDateTime } from "./project-utils";
import { ProjectMonitorMetricChart } from "./ProjectMonitorMetricChart";
import {
  aggregatePodResources,
  formatCpuCores,
  formatMemoryBytes,
  formatMillicores,
  loadPodMonitorSeries,
  pickMonitorPod,
  probeSummary,
  resolveProjectMonitorClusterId,
  type MetricPoint,
  type MetricRange,
} from "./project-monitor-utils";

interface ProjectMonitorTabProps {
  workspaceId: string;
  project: ProjectVM;
  deploymentPage: DeploymentRunListVM | null;
  active: boolean;
  refreshKey?: number;
}

const RANGE_OPTIONS: { value: MetricRange; label: string }[] = [
  { value: "5m", label: "近 5 分钟" },
  { value: "15m", label: "近 15 分钟" },
  { value: "1h", label: "近 1 小时" },
  { value: "6h", label: "近 6 小时" },
];

function replicaSummary(deployment: ProjectRuntimeOverviewVM["deployment"]) {
  if (!deployment) return "—";
  const available = deployment.available_replicas ?? 0;
  const total = deployment.replicas ?? 0;
  const failed = deployment.unavailable_replicas ?? 0;
  if (failed > 0) return `${available}/${total} 运行中，${failed} 个异常`;
  return `${available}/${total} 运行中`;
}

function phaseBadgeClass(phase: string) {
  if (phase === "Running" || phase === "Succeeded") return "badge-success";
  if (phase === "Pending") return "badge-warning";
  return "badge-warning";
}

export function ProjectMonitorTab({
  workspaceId,
  project,
  deploymentPage,
  active,
  refreshKey = 0,
}: ProjectMonitorTabProps) {
  const [overview, setOverview] = useState<ProjectRuntimeOverviewVM | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [range, setRange] = useState<MetricRange>("1h");
  const [selectedPodId, setSelectedPodId] = useState<string>("");
  const [seriesLoading, setSeriesLoading] = useState(false);
  const [prometheusHint, setPrometheusHint] = useState<string | null>(null);
  const [cpuSeries, setCpuSeries] = useState<MetricPoint[]>([]);
  const [memorySeries, setMemorySeries] = useState<MetricPoint[]>([]);

  const clusterId = useMemo(
    () => resolveProjectMonitorClusterId(project, deploymentPage),
    [project, deploymentPage],
  );

  const pods = overview?.pods ?? [];
  const selectedPod = useMemo(() => {
    if (!pods.length) return null;
    return pods.find((pod) => pod.id === selectedPodId) ?? pickMonitorPod(pods);
  }, [pods, selectedPodId]);

  const loadOverview = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await cluster.getProjectRuntimeOverview(project.id, workspaceId);
      setOverview(data);
    } catch (err) {
      setOverview(null);
      setError(err instanceof BffError ? err.message : "监控数据加载失败");
    } finally {
      setLoading(false);
    }
  }, [project.id, workspaceId]);

  useEffect(() => {
    if (!active) return;
    void loadOverview();
  }, [active, loadOverview, refreshKey]);

  useEffect(() => {
    if (!active || pods.length === 0) return;
    setSelectedPodId((current) => {
      if (current && pods.some((pod) => pod.id === current)) return current;
      return pickMonitorPod(pods)?.id ?? "";
    });
  }, [active, pods]);

  useEffect(() => {
    if (!active || !clusterId || !selectedPod) {
      setCpuSeries([]);
      setMemorySeries([]);
      setPrometheusHint(null);
      return;
    }

    const controller = new AbortController();
    setSeriesLoading(true);
    setPrometheusHint(null);

    void loadPodMonitorSeries(clusterId, selectedPod, range, controller.signal)
      .then((result) => {
        if (controller.signal.aborted) return;
        setCpuSeries(result.cpu);
        setMemorySeries(result.memory);
        if (!result.prometheusAvailable) {
          setPrometheusHint("Prometheus 未配置或不可用，仅展示 Metrics Server 快照。");
        }
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setCpuSeries([]);
        setMemorySeries([]);
        setPrometheusHint("Prometheus 趋势数据不可用，仅展示 Metrics Server 快照。");
      })
      .finally(() => {
        if (!controller.signal.aborted) setSeriesLoading(false);
      });

    return () => controller.abort();
  }, [active, clusterId, selectedPod, range, refreshKey]);

  const totals = useMemo(() => aggregatePodResources(pods), [pods]);
  const deployment = overview?.deployment;

  if (!active) return null;

  if (loading && !overview) {
    return <K8sLoadingState label="加载监控数据…" />;
  }

  if (error) {
    return (
      <div className="project-monitor-page">
        <InlineAlert tone="error">{error}</InlineAlert>
      </div>
    );
  }

  if (!clusterId) {
    return (
      <K8sEmptyState
        variant="empty"
        title="暂无可监控的运行实例"
        description="请先完成首次发布，系统将基于运行 Pod 展示 CPU / 内存监控。"
      />
    );
  }

  if (pods.length === 0) {
    return (
      <K8sEmptyState
        variant="empty"
        title="暂无运行 Pod"
        description="请确认 Deployment 已成功调度，或稍后再查看监控数据。"
      />
    );
  }

  return (
    <div className="project-monitor-page">
      <header className="project-monitor-toolbar">
        <div className="project-monitor-toolbar-copy">
          <h3 className="project-monitor-title">运行监控</h3>
          <p className="project-monitor-subtitle">
            基于 Metrics Server 快照与 Prometheus 趋势（如已配置）展示项目运行状态。
          </p>
        </div>
        <div className="project-monitor-toolbar-actions">
          <label className="project-monitor-range">
            <span>时间范围</span>
            <select
              className="input project-monitor-range-select"
              value={range}
              onChange={(event) => setRange(event.target.value as MetricRange)}
            >
              {RANGE_OPTIONS.map((option) => (
                <option key={option.value} value={option.value}>
                  {option.label}
                </option>
              ))}
            </select>
          </label>
          <label className="project-monitor-range">
            <span>Pod</span>
            <select
              className="input project-monitor-range-select"
              value={selectedPod?.id ?? ""}
              onChange={(event) => setSelectedPodId(event.target.value)}
            >
              {pods.map((pod) => (
                <option key={pod.id} value={pod.id}>
                  {pod.name} ({pod.phase})
                </option>
              ))}
            </select>
          </label>
          <button type="button" className="btn-secondary text-sm" onClick={() => void loadOverview()}>
            刷新
          </button>
        </div>
      </header>

      {prometheusHint ? <InlineAlert tone="warning">{prometheusHint}</InlineAlert> : null}

      <div className="project-monitor-summary">
        <article className="project-monitor-summary-card">
          <span>副本状态</span>
          <strong>{replicaSummary(deployment)}</strong>
        </article>
        <article className="project-monitor-summary-card">
          <span>健康检查</span>
          <strong>{probeSummary(overview?.health_probe)}</strong>
        </article>
        <article className="project-monitor-summary-card">
          <span>CPU 合计</span>
          <strong>{formatMillicores(totals.cpuMillicores)} 核</strong>
        </article>
        <article className="project-monitor-summary-card">
          <span>内存合计</span>
          <strong>{formatMemoryBytes(totals.memoryBytes)}</strong>
        </article>
      </div>

      <div className="project-monitor-charts">
        <ProjectMonitorMetricChart
          title="CPU 使用率"
          subtitle={selectedPod ? `${selectedPod.name} · ${RANGE_OPTIONS.find((item) => item.value === range)?.label}` : undefined}
          points={cpuSeries}
          tone="cpu"
          formatValue={formatCpuCores}
        />
        <ProjectMonitorMetricChart
          title="内存使用"
          subtitle={selectedPod ? `${selectedPod.name} · ${RANGE_OPTIONS.find((item) => item.value === range)?.label}` : undefined}
          points={memorySeries}
          tone="memory"
          formatValue={formatMemoryBytes}
        />
      </div>

      {seriesLoading ? <p className="project-monitor-series-loading">趋势数据更新中…</p> : null}

      <section className="project-runtime-card">
        <header className="project-runtime-card-header">
          <h4 className="project-runtime-card-title">Pod 资源快照</h4>
        </header>
        <div className="project-runtime-table-wrap">
          <table className="project-runtime-table">
            <thead>
              <tr>
                <th>名称</th>
                <th>节点</th>
                <th>状态</th>
                <th>CPU（核）</th>
                <th>内存</th>
                <th>重启</th>
                <th>启动时间</th>
              </tr>
            </thead>
            <tbody>
              {pods.map((pod: PodVM) => (
                <tr key={pod.id} className={pod.id === selectedPod?.id ? "is-selected" : undefined}>
                  <td>
                    <button
                      type="button"
                      className="project-runtime-pod-link"
                      onClick={() => setSelectedPodId(pod.id)}
                    >
                      {pod.name}
                    </button>
                    {pod.message ? (
                      <div className="project-runtime-pod-message">{pod.message}</div>
                    ) : null}
                  </td>
                  <td>{pod.node || "-"}</td>
                  <td>
                    <span className={`badge ${phaseBadgeClass(pod.phase)}`}>{pod.phase}</span>
                  </td>
                  <td>{formatMillicores(pod.cpu_millicores ?? 0)}</td>
                  <td>{formatMemoryBytes(pod.memory_bytes ?? 0)}</td>
                  <td>{pod.restarts}</td>
                  <td>{pod.started_at ? formatDateTime(pod.started_at) : "-"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>
    </div>
  );
}
