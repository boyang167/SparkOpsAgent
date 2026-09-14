import type { DeploymentRunListVM } from "@sparkops/contracts/delivery";
import type { PodVM, ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { k8sFetch } from "@/modules/plugins/kubernetes/api";
import { METRIC_TEMPLATES } from "@/modules/plugins/kubernetes/specialized/WorkloadPodActions.helpers";
import { latestSuccessfulDeployment } from "./project-deployment-state";

export type MetricRange = "5m" | "15m" | "1h" | "6h";

export interface MetricPoint {
  timestamp: number;
  value: number;
}

export function resolveProjectMonitorClusterId(
  project: ProjectVM,
  deploymentPage: DeploymentRunListVM | null | undefined,
): string | null {
  const latest = latestSuccessfulDeployment(deploymentPage ?? null);
  if (latest?.cluster_id?.trim()) return latest.cluster_id;
  if (project.default_cluster_id?.trim()) return project.default_cluster_id;
  return null;
}

export function pickMonitorPod(pods: PodVM[]): PodVM | null {
  if (pods.length === 0) return null;
  return pods.find((pod) => pod.phase === "Running") ?? pods[0] ?? null;
}

export function aggregatePodResources(pods: PodVM[]) {
  return pods.reduce(
    (acc, pod) => ({
      cpuMillicores: acc.cpuMillicores + (pod.cpu_millicores ?? 0),
      memoryBytes: acc.memoryBytes + (pod.memory_bytes ?? 0),
    }),
    { cpuMillicores: 0, memoryBytes: 0 },
  );
}

export function probeSummary(probe: ProjectRuntimeOverviewVM["health_probe"]) {
  if (!probe?.configured) return "未配置";
  const checks: string[] = [];
  if (probe.readiness) checks.push("Readiness");
  if (probe.liveness) checks.push("Liveness");
  const protocol = probe.protocol || "TCP";
  const target = probe.path?.trim()
    ? `${protocol} ${probe.path}`
    : probe.port
      ? `${protocol}:${probe.port}`
      : protocol;
  return `${target}${checks.length ? ` · ${checks.join(" + ")}` : ""}`;
}

export function parsePrometheusMatrix(body: unknown): MetricPoint[] {
  if (!body || typeof body !== "object") return [];
  const result = (body as { data?: { result?: Array<{ values?: unknown[] }> } }).data?.result;
  if (!Array.isArray(result) || result.length === 0) return [];
  const values = result[0]?.values;
  if (!Array.isArray(values)) return [];

  return values
    .map((entry) => {
      if (!Array.isArray(entry) || entry.length < 2) return null;
      const timestamp = Number(entry[0]);
      const value = Number(entry[1]);
      if (!Number.isFinite(timestamp) || !Number.isFinite(value)) return null;
      return { timestamp, value };
    })
    .filter((item): item is MetricPoint => item !== null);
}

export async function fetchPodMetricSeries(
  clusterId: string,
  namespace: string,
  podName: string,
  template: string,
  range: MetricRange,
  signal?: AbortSignal,
): Promise<MetricPoint[]> {
  const body = await k8sFetch<unknown>(
    `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/metrics`,
    {
      method: "POST",
      signal,
      body: JSON.stringify({
        template,
        range,
        namespace,
        pod: podName,
      }),
    },
  );
  return parsePrometheusMatrix(body);
}

export async function loadPodMonitorSeries(
  clusterId: string,
  pod: PodVM,
  range: MetricRange,
  signal?: AbortSignal,
) {
  const namespace = pod.namespace;
  const podName = pod.name;
  const [cpu, memory] = await Promise.allSettled([
    fetchPodMetricSeries(clusterId, namespace, podName, METRIC_TEMPLATES.cpu, range, signal),
    fetchPodMetricSeries(clusterId, namespace, podName, METRIC_TEMPLATES.memory, range, signal),
  ]);

  return {
    cpu: cpu.status === "fulfilled" ? cpu.value : [],
    memory: memory.status === "fulfilled" ? memory.value : [],
    prometheusAvailable: cpu.status === "fulfilled" || memory.status === "fulfilled",
  };
}

export function formatCpuCores(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0";
  if (value >= 1) return `${value.toFixed(2).replace(/\.?0+$/, "")} 核`;
  return `${Math.round(value * 1000)}m`;
}

export function formatMemoryBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return "0 MiB";
  const mib = value / (1024 * 1024);
  if (mib < 0.01) return "<0.01 MiB";
  if (mib < 10) return `${mib.toFixed(2)} MiB`;
  return `${mib.toFixed(1)} MiB`;
}

export function formatMillicores(millicores: number) {
  if (!Number.isFinite(millicores) || millicores <= 0) return "0";
  const cores = millicores / 1000;
  if (cores >= 1) return cores.toFixed(2).replace(/\.?0+$/, "");
  if (cores >= 0.01) return cores.toFixed(3).replace(/\.?0+$/, "");
  return `${millicores}m`;
}
