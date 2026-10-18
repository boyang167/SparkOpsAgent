export type NodeAddress = { type?: string; address?: string };

export type NodeCondition = {
  type?: string;
  status?: string;
  lastTransitionTime?: string;
  reason?: string;
  message?: string;
};

export type QuantityMap = Record<string, string>;

export type NodeObject = {
  metadata?: {
    name?: string;
    uid?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
    annotations?: Record<string, string>;
  };
  spec?: {
    podCIDR?: string;
    unschedulable?: boolean;
    taints?: { key?: string; value?: string; effect?: string }[];
  };
  status?: {
    addresses?: NodeAddress[];
    conditions?: NodeCondition[];
    capacity?: QuantityMap;
    allocatable?: QuantityMap;
    nodeInfo?: {
      kubeletVersion?: string;
      kubeProxyVersion?: string;
      containerRuntimeVersion?: string;
      osImage?: string;
      kernelVersion?: string;
      machineID?: string;
      systemUUID?: string;
      bootID?: string;
      operatingSystem?: string;
      architecture?: string;
    };
  };
};

export type ContainerStatus = {
  name?: string;
  ready?: boolean;
  restartCount?: number;
  state?: {
    waiting?: { reason?: string };
    running?: { startedAt?: string };
    terminated?: { reason?: string };
  };
};

export type PodObject = {
  metadata?: {
    name?: string;
    namespace?: string;
    creationTimestamp?: string;
    labels?: Record<string, string>;
    ownerReferences?: { kind?: string }[];
  };
  spec?: {
    nodeName?: string;
    containers?: { name?: string; resources?: { requests?: QuantityMap; limits?: QuantityMap } }[];
    initContainers?: { name?: string; resources?: { requests?: QuantityMap; limits?: QuantityMap } }[];
  };
  status?: {
    phase?: string;
    podIP?: string;
    hostIP?: string;
    containerStatuses?: ContainerStatus[];
  };
};

export type NodeMetrics = {
  metadata?: { name?: string };
  usage?: QuantityMap;
};

/** Convert a Kubernetes CPU quantity to millicores. */
export function cpuToMillicores(value?: string): number {
  const raw = value?.trim();
  if (!raw) return 0;
  if (raw.endsWith("n")) return Number(raw.slice(0, -1)) / 1_000_000;
  if (raw.endsWith("u")) return Number(raw.slice(0, -1)) / 1_000;
  if (raw.endsWith("m")) return Number(raw.slice(0, -1));
  const n = Number(raw);
  return Number.isFinite(n) ? n * 1000 : 0;
}

/** Convert a Kubernetes memory quantity to bytes. */
export function memoryToBytes(value?: string): number {
  const raw = value?.trim();
  if (!raw) return 0;
  const match = raw.match(/^([0-9.]+)([A-Za-z]+)?$/);
  if (!match) return 0;
  const n = Number(match[1]);
  if (!Number.isFinite(n)) return 0;
  const unit = match[2] ?? "";
  const binary: Record<string, number> = {
    Ki: 1024,
    Mi: 1024 ** 2,
    Gi: 1024 ** 3,
    Ti: 1024 ** 4,
  };
  if (unit in binary) return n * binary[unit];
  if (unit === "k" || unit === "K") return n * 1000;
  if (unit === "M") return n * 1_000_000;
  if (unit === "G") return n * 1_000_000_000;
  return n;
}

export function nodeInternalIP(node: NodeObject): string {
  return node.status?.addresses?.find((item) => item.type === "InternalIP")?.address ?? "-";
}

export function nodeStatus(node: NodeObject): string {
  const ready = node.status?.conditions?.find((item) => item.type === "Ready");
  let status = ready?.status === "True" ? "Ready" : ready?.status === "False" ? "NotReady" : "Unknown";
  if (node.spec?.unschedulable) status += ", SchedulingDisabled";
  return status;
}

const INVERTED_NODE_CONDITIONS = new Set([
  "MemoryPressure",
  "DiskPressure",
  "PIDPressure",
  "NetworkUnavailable",
]);

export function nodeConditionTone(type?: string, status?: string): "success" | "danger" | "neutral" {
  if (!status) return "neutral";
  const isTrue = status === "True";
  if (type === "Ready") return isTrue ? "success" : "danger";
  if (type && INVERTED_NODE_CONDITIONS.has(type)) return isTrue ? "danger" : "success";
  return isTrue ? "success" : "neutral";
}

export function nodeRoles(labels?: Record<string, string>): string[] {
  if (!labels) return ["Worker"];
  const roles = Object.keys(labels)
    .filter((key) => key.startsWith("node-role.kubernetes.io/"))
    .map((key) => key.slice("node-role.kubernetes.io/".length))
    .filter(Boolean);
  if (labels["kubernetes.io/role"]) roles.push(labels["kubernetes.io/role"]);
  if (roles.length === 0) roles.push("Worker");
  return [...new Set(roles)];
}

export function formatCores(millicores: number): string {
  return (millicores / 1000).toFixed(2);
}

export function formatGi(bytes: number): string {
  return (bytes / 1024 ** 3).toFixed(2);
}

export function percent(part: number, whole: number): number {
  if (whole <= 0) return 0;
  return Number(((part / whole) * 100).toFixed(2));
}

export function sumPodResources(pods: PodObject[]): {
  cpuReq: number;
  cpuLim: number;
  memReq: number;
  memLim: number;
} {
  let cpuReq = 0;
  let cpuLim = 0;
  let memReq = 0;
  let memLim = 0;
  for (const pod of pods) {
    const containers = [...(pod.spec?.containers ?? []), ...(pod.spec?.initContainers ?? [])];
    for (const container of containers) {
      cpuReq += cpuToMillicores(container.resources?.requests?.cpu);
      cpuLim += cpuToMillicores(container.resources?.limits?.cpu);
      memReq += memoryToBytes(container.resources?.requests?.memory);
      memLim += memoryToBytes(container.resources?.limits?.memory);
    }
  }
  return { cpuReq, cpuLim, memReq, memLim };
}

export function podResourceSummary(pod: PodObject): { cpu: string; memory: string } {
  const { cpuReq, cpuLim, memReq, memLim } = sumPodResources([pod]);
  return {
    cpu: `${formatCores(cpuReq)} / ${formatCores(cpuLim)} 核`,
    memory: `${formatGi(memReq)} / ${formatGi(memLim)} Gi`,
  };
}

export function podContainerNames(pod: PodObject): string[] {
  const fromSpec = (pod.spec?.containers ?? []).map((item) => item.name).filter(Boolean) as string[];
  if (fromSpec.length > 0) return fromSpec;
  return (pod.status?.containerStatuses ?? []).map((item) => item.name).filter(Boolean) as string[];
}

export type NodeMetricView = {
  percent: number;
  usedLabel: string;
  capLabel: string;
  overcommit?: boolean;
};

export function nodeResourceView(
  node: NodeObject,
  pods: PodObject[],
  metrics?: NodeMetrics,
): { cpu: NodeMetricView; memory: NodeMetricView } {
  const cpuCap = cpuToMillicores(node.status?.allocatable?.cpu ?? node.status?.capacity?.cpu);
  const memCap = memoryToBytes(node.status?.allocatable?.memory ?? node.status?.capacity?.memory);
  const usageCpu = cpuToMillicores(metrics?.usage?.cpu);
  const usageMem = memoryToBytes(metrics?.usage?.memory);
  const { cpuReq, cpuLim, memReq, memLim } = sumPodResources(pods);
  const cpuUsed = usageCpu > 0 ? usageCpu : cpuReq;
  const memUsed = usageMem > 0 ? usageMem : memReq;
  return {
    cpu: {
      percent: percent(cpuUsed, cpuCap),
      usedLabel: `${formatCores(cpuUsed)} / ${formatCores(cpuCap)} 核`,
      capLabel: `申请 ${percent(cpuReq, cpuCap).toFixed(2)}% / 限制 ${percent(cpuLim, cpuCap).toFixed(2)}%`,
      overcommit: percent(cpuLim, cpuCap) > 100,
    },
    memory: {
      percent: percent(memUsed, memCap),
      usedLabel: `${formatGi(memUsed)} / ${formatGi(memCap)} Gi`,
      capLabel: `申请 ${percent(memReq, memCap).toFixed(2)}% / 限制 ${percent(memLim, memCap).toFixed(2)}%`,
      overcommit: percent(memLim, memCap) > 100,
    },
  };
}

export function formatNodeAge(timestamp?: string, now = Date.now()): string {
  if (!timestamp) return "-";
  const created = new Date(timestamp).getTime();
  if (Number.isNaN(created)) return "-";
  const delta = Math.max(0, now - created);
  const minutes = Math.floor(delta / 60_000);
  if (minutes < 1) return "刚刚";
  if (minutes < 60) return `${minutes} 分钟`;
  const hours = Math.floor(minutes / 60);
  if (hours < 24) return `${hours} 小时`;
  const days = Math.floor(hours / 24);
  return `${days} 天`;
}

export function formatDateTime(timestamp?: string): string {
  if (!timestamp) return "-";
  const d = new Date(timestamp);
  if (Number.isNaN(d.getTime())) return "-";
  const pad = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
}

export function podPhase(pod: PodObject): string {
  return pod.status?.phase || "Unknown";
}

export function podRestartCount(pod: PodObject): number {
  return (pod.status?.containerStatuses ?? []).reduce((sum, item) => sum + (item.restartCount ?? 0), 0);
}

export function isEvictablePod(pod: PodObject): boolean {
  const owners = pod.metadata?.ownerReferences ?? [];
  if (owners.some((item) => item.kind === "DaemonSet")) return false;
  if (pod.metadata?.name?.startsWith("node-shell") || pod.metadata?.name?.includes("nodetty")) return false;
  return true;
}
