import type { ResourceDef } from "./catalog";
import { formatDateTime } from "./node-view";

export const WORKLOAD_TABS = [
  "Deployment",
  "DaemonSet",
  "StatefulSet",
  "Job",
  "CronJob",
  "Pod",
] as const;

export function replicaBadge(
  ready?: number,
  desired?: number,
): { text: string; tone: "ok" | "bad" } {
  const r = ready ?? 0;
  const d = desired ?? 0;
  return {
    text: `${r}/${d}`,
    tone: r === d ? "ok" : "bad",
  };
}

export function daemonSetCounts(obj: Record<string, unknown>): {
  ready: number;
  current: number;
  desired: number;
} {
  const status = (obj.status ?? {}) as Record<string, unknown>;
  return {
    ready: Number(status.numberReady ?? 0),
    current: Number(status.currentNumberScheduled ?? 0),
    desired: Number(status.desiredNumberScheduled ?? 0),
  };
}

function formatDuration(startTime?: string, endTime = Date.now()): string {
  if (!startTime) return "-";
  const start = new Date(startTime).getTime();
  if (Number.isNaN(start)) return "-";
  const delta = Math.max(0, endTime - start);
  const minutes = Math.floor(delta / 60_000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时`;
  const days = Math.floor(hours / 24);
  return `${days} 天`;
}

export function jobStatus(obj: Record<string, unknown>): {
  active: number;
  succeeded: number;
  failed: number;
  duration: string;
} {
  const status = (obj.status ?? {}) as Record<string, unknown>;
  const completionTime = status.completionTime as string | undefined;
  const endTime = completionTime ? new Date(completionTime).getTime() : Date.now();
  return {
    active: Number(status.active ?? 0),
    succeeded: Number(status.succeeded ?? 0),
    failed: Number(status.failed ?? 0),
    duration: formatDuration(status.startTime as string | undefined, endTime),
  };
}

export function cronJobView(obj: Record<string, unknown>): {
  schedule: string;
  lastSchedule: string;
  suspend: boolean;
} {
  const spec = (obj.spec ?? {}) as Record<string, unknown>;
  const status = (obj.status ?? {}) as Record<string, unknown>;
  return {
    schedule: String(spec.schedule ?? "-"),
    lastSchedule: formatDateTime(status.lastScheduleTime as string | undefined),
    suspend: Boolean(spec.suspend),
  };
}

export function podPhaseBadge(phase?: string): {
  text: string;
  tone: "ok" | "bad" | "muted";
} {
  if (phase === "Running") return { text: "Running", tone: "ok" };
  if (phase === "Succeeded") return { text: "Completed", tone: "ok" };
  if (!phase) return { text: "-", tone: "muted" };
  return { text: phase, tone: "bad" };
}

export function clampPageToTotal(page: number, total: number, pageSize: number): number {
  const safePageSize = Math.max(1, Math.floor(pageSize));
  const totalPages = Math.max(1, Math.ceil(Math.max(0, total) / safePageSize));
  return Math.min(Math.max(1, Math.floor(page)), totalPages);
}

export function validateExistingIdentity(original: Record<string, unknown>, next: Record<string, unknown>): string | null {
  const originalIdentity = resourceIdentity(original);
  const nextIdentity = resourceIdentity(next);
  for (const key of ["apiVersion", "kind", "metadata.name", "metadata.namespace"] as const) {
    if (originalIdentity[key] !== nextIdentity[key]) {
      return `不能修改 ${key}：打开时为「${originalIdentity[key] || "空"}」，当前为「${nextIdentity[key] || "空"}」。请保持资源身份不变。`;
    }
  }
  return null;
}

export function validateCreateIdentity(def: ResourceDef, next: Record<string, unknown>): string | null {
  const identity = resourceIdentity(next);
  const expectedApiVersion = resourceApiVersion(def);
  if (identity.kind !== def.kind) {
    return `不能通过 ${def.kind} 新增弹窗创建其他 Kind：请保持 kind 为「${def.kind}」。`;
  }
  if (identity.apiVersion !== expectedApiVersion) {
    return `不能修改 apiVersion：${def.kind} 必须使用「${expectedApiVersion}」。`;
  }
  return null;
}

const WORKLOAD_API_VERSION: Record<string, string> = {
  Deployment: "apps/v1",
  DaemonSet: "apps/v1",
  StatefulSet: "apps/v1",
  Job: "batch/v1",
  CronJob: "batch/v1",
  Pod: "v1",
};

export function emptyWorkloadTemplate(
  kind: string,
  namespace?: string,
): Record<string, unknown> {
  const tpl: Record<string, unknown> = {
    apiVersion: WORKLOAD_API_VERSION[kind] ?? "v1",
    kind,
    metadata: namespace ? { namespace } : {},
  };
  if (kind === "Deployment") {
    tpl.spec = {
      replicas: 1,
      selector: {},
      template: {},
    };
  }
  return tpl;
}

function resourceIdentity(obj: Record<string, unknown>): Record<"apiVersion" | "kind" | "metadata.name" | "metadata.namespace", string> {
  const metadata = obj.metadata && typeof obj.metadata === "object" && !Array.isArray(obj.metadata) ? (obj.metadata as Record<string, unknown>) : {};
  return {
    apiVersion: stringValue(obj.apiVersion),
    kind: stringValue(obj.kind),
    "metadata.name": stringValue(metadata.name),
    "metadata.namespace": stringValue(metadata.namespace),
  };
}

function resourceApiVersion(def: ResourceDef): string {
  return def.group ? `${def.group}/${def.version}` : def.version;
}

function stringValue(value: unknown): string {
  return typeof value === "string" ? value.trim() : "";
}
