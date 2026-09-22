import type { ProductWorkflowRunSummaryVM, ProductWorkflowVM, WorkflowStageVM } from "@sparkops/contracts/product";
import { jobTypeLabel } from "./workflow-editor-utils";

export type SparklineTone = "success" | "failed" | "running" | "pending" | "empty";

export interface WorkflowRunStats {
  sparkline: SparklineTone[];
  successRate: number | null;
  avgDurationMs: number | null;
  lastRunAt: string | null;
  lastRunStatus: string | null;
}

const FAVORITES_KEY = "sparkops:workflow-favorites";

export function readWorkflowFavorites(): Set<string> {
  if (typeof window === "undefined") return new Set();
  try {
    const raw = window.localStorage.getItem(FAVORITES_KEY);
    if (!raw) return new Set();
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return new Set();
    return new Set(parsed.filter((item): item is string => typeof item === "string"));
  } catch {
    return new Set();
  }
}

export function writeWorkflowFavorites(ids: Set<string>) {
  if (typeof window === "undefined") return;
  window.localStorage.setItem(FAVORITES_KEY, JSON.stringify([...ids]));
}

export function stagePathLabels(stages: WorkflowStageVM[] | undefined): string[] {
  if (!stages?.length) return [];
  return stages.map((stage) => {
    const firstJob = stage.jobs[0];
    if (firstJob?.type) return jobTypeLabel(firstJob.type);
    return stage.name.trim() || "阶段";
  });
}

export function runStatusSparkTone(status: string): SparklineTone {
  switch (status) {
    case "success":
      return "success";
    case "failed":
      return "failed";
    case "running":
    case "paused":
      return "running";
    case "pending":
      return "pending";
    default:
      return "empty";
  }
}

export function computeRunStats(runs: ProductWorkflowRunSummaryVM[]): WorkflowRunStats {
  const ordered = [...runs].sort((a, b) => {
    const at = a.created_at ? Date.parse(a.created_at) : 0;
    const bt = b.created_at ? Date.parse(b.created_at) : 0;
    return at - bt;
  });
  const recent = ordered.slice(-20);
  const sparkline = recent.map((run) => runStatusSparkTone(run.status));
  while (sparkline.length < 20) sparkline.unshift("empty");

  const finished = ordered.filter((run) => run.status === "success" || run.status === "failed");
  const successCount = finished.filter((run) => run.status === "success").length;
  const successRate = finished.length > 0 ? Math.round((successCount / finished.length) * 10000) / 100 : null;

  const durations = finished
    .map((run) => run.duration_ms ?? 0)
    .filter((value) => value > 0);
  const avgDurationMs =
    durations.length > 0 ? Math.round(durations.reduce((sum, value) => sum + value, 0) / durations.length) : null;

  const last = ordered[ordered.length - 1];
  return {
    sparkline,
    successRate,
    avgDurationMs,
    lastRunAt: last?.created_at ?? null,
    lastRunStatus: last?.status ?? null,
  };
}

export function formatDurationMs(ms: number | null): string {
  if (ms == null || ms <= 0) return "—";
  const totalSeconds = Math.round(ms / 1000);
  const minutes = Math.floor(totalSeconds / 60);
  const seconds = totalSeconds % 60;
  if (minutes <= 0) return `${seconds} 秒`;
  if (minutes < 60) return seconds > 0 ? `${minutes} 分 ${seconds} 秒` : `${minutes} 分`;
  const hours = Math.floor(minutes / 60);
  const restMinutes = minutes % 60;
  return restMinutes > 0 ? `${hours} 小时 ${restMinutes} 分` : `${hours} 小时`;
}

export function formatLastRun(at: string | null): string {
  if (!at) return "—";
  const date = new Date(at);
  if (Number.isNaN(date.getTime())) return "—";
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  const hour = String(date.getHours()).padStart(2, "0");
  const minute = String(date.getMinutes()).padStart(2, "0");
  return `${month}-${day} ${hour}:${minute}`;
}

export function workflowStagePathText(detail: ProductWorkflowVM | null, stageCount: number): string {
  const labels = stagePathLabels(detail?.stages);
  if (labels.length > 0) return labels.join(" > ");
  if (stageCount > 0) return `${stageCount} 个阶段`;
  return "未配置阶段";
}

export function successRateTone(rate: number | null): "is-good" | "is-mid" | "is-low" | "" {
  if (rate == null) return "";
  if (rate >= 80) return "is-good";
  if (rate >= 50) return "is-mid";
  return "is-low";
}

export function successRateBadgeClass(rate: number | null): string {
  const tone = successRateTone(rate);
  if (tone === "is-good") return "badge-success";
  if (tone === "is-mid") return "badge-warning";
  if (tone === "is-low") return "badge-danger";
  return "badge-neutral";
}

export function paginationPages(current: number, total: number): number[] {
  if (total <= 1) return total === 1 ? [1] : [];
  if (total <= 7) return Array.from({ length: total }, (_, i) => i + 1);
  const set = new Set<number>([1, total, current, current - 1, current + 1]);
  return [...set].filter((p) => p >= 1 && p <= total).sort((a, b) => a - b);
}
