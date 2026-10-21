import { k8sFetch } from "./api";

export type DiagnosisTarget = {
  clusterId: string;
  namespace: string;
  pod: string;
  container: string;
  processId: string;
};

export type DiagnosisRecord = {
  id?: string;
  cluster_id?: string;
  namespace?: string;
  pod_name?: string;
  container?: string;
  local_port?: number;
  remote_port?: number;
  status?: string;
};

export type ProcessOption = {
  name: string;
  value: string;
};

export type DiagnosisTab =
  | "dashboard"
  | "threads"
  | "jvm"
  | "perfcounter"
  | "sysprop"
  | "thread_stack"
  | "profiler"
  | "trace"
  | "watch"
  | "monitor"
  | "sourcecode"
  | "getstatic";

export const DIAGNOSIS_TABS: { id: DiagnosisTab; label: string; command?: string }[] = [
  { id: "dashboard", label: "控制面板" },
  { id: "threads", label: "线程清单", command: "thread" },
  { id: "jvm", label: "JVM信息", command: "jvm" },
  { id: "perfcounter", label: "性能计数器", command: "perfcounter" },
  { id: "sysprop", label: "系统信息", command: "sysprop" },
  { id: "thread_stack", label: "线程堆栈", command: "thread -n 5" },
  { id: "profiler", label: "火焰图", command: "profiler" },
  { id: "trace", label: "方法追踪", command: "trace" },
  { id: "watch", label: "方法监测", command: "watch" },
  { id: "monitor", label: "方法监控", command: "monitor" },
  { id: "sourcecode", label: "源码查看", command: "jad" },
  { id: "getstatic", label: "静态属性", command: "getstatic" },
];

export type NameValueRow = { name: string; value: string };

export type DashboardThread = {
  id?: number;
  name?: string;
  group?: string;
  priority?: number;
  state?: string;
  cpu?: number;
  deltaTime?: number;
  time?: number;
  interrupted?: boolean;
  daemon?: boolean;
};

export type DashboardMemory = {
  name?: string;
  type?: string;
  used?: number;
  total?: number;
  max?: number;
  usage?: number;
};

export type DashboardGC = {
  name?: string;
  collectionCount?: number;
  collectionTime?: number;
};

export type DashboardRuntime = {
  osName?: string;
  osVersion?: string;
  javaVersion?: string;
  javaHome?: string;
  systemProperties?: Record<string, string>;
  inputArguments?: string[];
  uptime?: number;
  startTime?: number;
};

function diagnosisBase(clusterId: string, suffix: string) {
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/diagnosis/${suffix}`;
}

export function diagnosisTargetPayload(target: DiagnosisTarget) {
  return {
    namespace: target.namespace,
    pod: target.pod,
    container: target.container,
    process_id: target.processId,
  };
}

export async function diagnosisInstall(clusterId: string, target: DiagnosisTarget) {
  return k8sFetch<{ diagnosis?: DiagnosisRecord }>(diagnosisBase(clusterId, "install"), {
    method: "POST",
    body: JSON.stringify(diagnosisTargetPayload(target)),
  });
}

export async function diagnosisUninstall(clusterId: string, target: DiagnosisTarget) {
  return k8sFetch<{ ok?: boolean }>(diagnosisBase(clusterId, "uninstall"), {
    method: "DELETE",
    body: JSON.stringify(diagnosisTargetPayload(target)),
  });
}

export async function diagnosisGetByPod(clusterId: string, target: DiagnosisTarget) {
  return k8sFetch<{ diagnosis?: DiagnosisRecord }>(diagnosisBase(clusterId, "by-pod"), {
    method: "POST",
    body: JSON.stringify(diagnosisTargetPayload(target)),
  });
}

export async function diagnosisJps(clusterId: string, target: DiagnosisTarget) {
  return k8sFetch<{ diagnosis?: ProcessOption[] }>(diagnosisBase(clusterId, "jps"), {
    method: "POST",
    body: JSON.stringify(diagnosisTargetPayload(target)),
  });
}

export async function diagnosisProxy(clusterId: string, target: DiagnosisTarget, body: Record<string, unknown>) {
  return k8sFetch<{ arthas?: ArthasProxyResponse }>(diagnosisBase(clusterId, "proxy"), {
    method: "POST",
    body: JSON.stringify({ ...diagnosisTargetPayload(target), body }),
  });
}

export type ArthasProxyResponse = {
  consumerId?: string;
  sessionId?: string;
  state?: string;
  body?: {
    jobStatus?: string;
    results?: Array<Record<string, unknown>>;
  };
};

export function extractNameValuePairs(result: Record<string, unknown>): NameValueRow[] {
  const rows: NameValueRow[] = [];
  for (const [key, value] of Object.entries(result)) {
    if (key.endsWith("Info") && value && typeof value === "object") {
      for (const [name, nested] of Object.entries(value as Record<string, unknown>)) {
        if (nested && typeof nested === "object") {
          for (const [field, fieldValue] of Object.entries(nested as Record<string, unknown>)) {
            rows.push({ name: `${name}.${field}`, value: formatCell(fieldValue) });
          }
        } else {
          rows.push({ name, value: formatCell(nested) });
        }
      }
    }
  }
  return rows;
}

export function flattenArthasResults(response?: ArthasProxyResponse): NameValueRow[] {
  const rows: NameValueRow[] = [];
  for (const item of response?.body?.results ?? []) {
    rows.push(...extractNameValuePairs(item));
    for (const [key, value] of Object.entries(item)) {
      if (Array.isArray(value)) {
        value.forEach((entry, index) => {
          if (entry && typeof entry === "object") {
            for (const [field, fieldValue] of Object.entries(entry as Record<string, unknown>)) {
              rows.push({ name: `${key}[${index}].${field}`, value: formatCell(fieldValue) });
            }
          }
        });
      } else if (value && typeof value === "object") {
        for (const [field, fieldValue] of Object.entries(value as Record<string, unknown>)) {
          rows.push({ name: `${key}.${field}`, value: formatCell(fieldValue) });
        }
      } else if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
        rows.push({ name: key, value: formatCell(value) });
      }
    }
  }
  return rows;
}

export function dashboardFromProxy(response?: ArthasProxyResponse) {
  const result = response?.body?.results?.[0] ?? {};
  const memoryInfo = (result.memoryInfo ?? {}) as Record<string, unknown>;
  const heap = Array.isArray(memoryInfo.heap) ? memoryInfo.heap : [];
  const nonheap = Array.isArray(memoryInfo.nonheap) ? memoryInfo.nonheap : [];
  const bufferPool = Array.isArray(memoryInfo.buffer_pool) ? memoryInfo.buffer_pool : [];
  return {
    threads: (result.threads as DashboardThread[]) ?? [],
    memory: [...heap, ...nonheap, ...bufferPool] as DashboardMemory[],
    gcInfos: (result.gcInfos as DashboardGC[]) ?? [],
    runtimeInfo: (result.runtimeInfo as DashboardRuntime) ?? {},
  };
}

function formatCell(value: unknown): string {
  if (value == null) return "-";
  if (typeof value === "object") return JSON.stringify(value);
  return String(value);
}

export function threadStateSummary(threads: DashboardThread[]) {
  const summary = {
    SUM: threads.length,
    NEW: 0,
    RUNNABLE: 0,
    BLOCKED: 0,
    WAITING: 0,
    TIMED_WAITING: 0,
    TERMINATED: 0,
  };
  for (const thread of threads) {
    const state = String(thread.state ?? "").toUpperCase();
    if (state in summary) {
      summary[state as keyof typeof summary] += 1;
    }
  }
  return summary;
}

export function isTargetReady(target: DiagnosisTarget) {
  return Boolean(target.clusterId && target.namespace && target.pod && target.container);
}

export function execUrl(clusterId: string, target: DiagnosisTarget, command: string[]) {
  const params = new URLSearchParams();
  if (target.container) params.set("container", target.container);
  for (const part of command) params.append("command", part);
  return (
    `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}` +
    `/namespaces/${encodeURIComponent(target.namespace)}` +
    `/pods/${encodeURIComponent(target.pod)}/exec?${params.toString()}`
  );
}
