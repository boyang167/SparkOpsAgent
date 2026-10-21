import { k8sFetch } from "../api";
import { podContainerNames, type PodObject } from "../node-view";

export const METRIC_TEMPLATES = {
  cpu: "pod_cpu",
  memory: "pod_memory",
} as const;

type MetricKey = keyof typeof METRIC_TEMPLATES;

type MetricResponses = {
  cpu?: unknown;
  memory?: unknown;
  tcp?: unknown;
};

export type ContainerReadinessDot = {
  name: string;
  tone: "ready" | "bad" | "muted";
  title: string;
};

export function containerReadinessDots(pod: PodObject): ContainerReadinessDot[] {
  const statuses = new Map((pod.status?.containerStatuses ?? []).map((item) => [item.name, item]));
  const names = podContainerNames(pod);
  return names.map((name) => {
    const status = statuses.get(name);
    if (status?.ready) return { name, tone: "ready", title: `${name} ready` };
    const state = status?.state;
    if (state?.waiting) return { name, tone: "bad", title: `${name} waiting: ${state.waiting.reason ?? "unknown"}` };
    if (state?.terminated) return { name, tone: "muted", title: `${name} terminated: ${state.terminated.reason ?? "unknown"}` };
    return { name, tone: "muted", title: `${name} not ready` };
  });
}

export function formatPodMetricValue(value: unknown): string {
  const direct = metricSampleValue(value);
  return direct === undefined || direct === "" ? "-" : String(direct);
}

export function podMetricCell(metrics: MetricResponses): string {
  // The current BFF only exposes pod_cpu and pod_memory templates; keep TCP as an empty stable column.
  return `CPU ${formatPodMetricValue(metrics.cpu)} / MEM ${formatPodMetricValue(metrics.memory)} / TCP ${formatPodMetricValue(metrics.tcp)}`;
}

export function settledPodMetrics(
  entries: PromiseSettledResult<readonly [MetricKey, unknown]>[],
): MetricResponses {
  const metrics: MetricResponses = {};
  for (const entry of entries) {
    if (entry.status !== "fulfilled") continue;
    const [key, value] = entry.value;
    metrics[key] = value;
  }
  return metrics;
}

export async function withLimitedConcurrency<T, R>(
  items: readonly T[],
  limit: number,
  worker: (item: T, index: number) => Promise<R>,
  signal?: AbortSignal,
): Promise<R[]> {
  const concurrency = Math.max(1, Math.floor(limit));
  const results = new Array<R>(items.length);
  let nextIndex = 0;

  async function runNext(): Promise<void> {
    for (;;) {
      throwIfAborted(signal);
      const index = nextIndex;
      nextIndex += 1;
      if (index >= items.length) return;
      results[index] = await worker(items[index], index);
    }
  }

  await Promise.all(Array.from({ length: Math.min(concurrency, items.length) }, () => runNext()));
  return results;
}

export async function loadPodMetricCell(clusterId: string, pod: PodObject, signal?: AbortSignal): Promise<string> {
  const namespace = pod.metadata?.namespace;
  const podName = pod.metadata?.name;
  if (!namespace || !podName) return "-";
  const endpoint = `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/metrics`;
  const entries = await Promise.allSettled(
    metricTemplateEntries().map(async ([key, template]) => {
      const value = await k8sFetch<unknown>(endpoint, {
        method: "POST",
        signal,
        body: JSON.stringify({ template, range: "1h", namespace, pod: podName }),
      });
      return [key, value] as const;
    }),
  );
  return podMetricCell(settledPodMetrics(entries));
}

function metricTemplateEntries(): [MetricKey, string][] {
  return Object.entries(METRIC_TEMPLATES) as [MetricKey, string][];
}

function metricSampleValue(value: unknown): unknown {
  if (typeof value === "string" || typeof value === "number") return value;
  if (!value || typeof value !== "object") return undefined;
  if (Array.isArray(value)) {
    if (value.length === 2 && typeof value[0] === "number" && typeof value[1] === "string") return value[1];
    return value.length > 0 ? metricSampleValue(value[value.length - 1]) : undefined;
  }
  const record = value as Record<string, unknown>;
  if (Array.isArray(record.value)) return metricSampleValue(record.value);
  if (Array.isArray(record.values)) return metricSampleValue(record.values);
  const data = record.data;
  if (data && typeof data === "object" && !Array.isArray(data)) {
    const result = (data as Record<string, unknown>).result;
    if (Array.isArray(result) && result.length > 0) return metricSampleValue(result[0]);
    return undefined;
  }
  return undefined;
}

function throwIfAborted(signal?: AbortSignal) {
  if (!signal?.aborted) return;
  if (typeof DOMException !== "undefined") throw new DOMException("Aborted", "AbortError");
  const err = new Error("Aborted");
  err.name = "AbortError";
  throw err;
}
