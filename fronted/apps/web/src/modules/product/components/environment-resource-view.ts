type K8sMeta = {
  kind?: string;
  metadata?: { name?: string; namespace?: string; creationTimestamp?: string };
  spec?: Record<string, unknown>;
  status?: Record<string, unknown>;
};

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  return value as Record<string, unknown>;
}

function podTemplateSpec(item: K8sMeta): Record<string, unknown> | null {
  const spec = asRecord(item.spec);
  if (!spec) return null;
  if (item.kind === "Pod") return spec;
  if (item.kind === "CronJob") {
    const jobTemplate = asRecord(spec.jobTemplate);
    const jobSpec = asRecord(jobTemplate?.spec);
    const template = asRecord(jobSpec?.template);
    return asRecord(template?.spec);
  }
  const template = asRecord(spec.template);
  return asRecord(template?.spec);
}

export function resourceImages(item: K8sMeta): string {
  const podSpec = podTemplateSpec(item);
  if (!podSpec) return "—";
  const containers = Array.isArray(podSpec.containers) ? podSpec.containers : [];
  const images = containers
    .map((c) => (asRecord(c)?.image ? String(asRecord(c)?.image) : ""))
    .filter(Boolean);
  if (images.length === 0) return "—";
  return images.join(", ");
}

export function resourceReady(item: K8sMeta): string {
  const spec = asRecord(item.spec) ?? {};
  const status = asRecord(item.status) ?? {};
  const kind = item.kind ?? "";
  if (kind === "Deployment" || kind === "StatefulSet" || kind === "ReplicaSet") {
    return `${Number(status.readyReplicas ?? 0)}/${Number(spec.replicas ?? status.replicas ?? 0)}`;
  }
  if (kind === "DaemonSet") {
    return `${Number(status.numberReady ?? 0)}/${Number(status.desiredNumberScheduled ?? 0)}`;
  }
  if (kind === "Job") {
    return `${Number(status.succeeded ?? 0)}/${Number(spec.completions ?? 1)}`;
  }
  if (kind === "CronJob") {
    return spec.suspend ? "暂停" : "调度中";
  }
  if (kind === "Pod") {
    const containerStatuses = Array.isArray(status.containerStatuses) ? status.containerStatuses : [];
    const ready = containerStatuses.filter((c) => Boolean(asRecord(c)?.ready)).length;
    const total = containerStatuses.length || (Array.isArray(spec.containers) ? spec.containers.length : 0);
    return `${ready}/${total}`;
  }
  if (kind === "PersistentVolumeClaim") {
    return String(status.phase ?? "—");
  }
  if (kind === "Service") {
    return String(spec.type ?? "ClusterIP");
  }
  return "—";
}

export const ENV_RESOURCE_GROUPS = [
  { id: "workloads", label: "工作负载", kinds: ["Deployment", "StatefulSet", "DaemonSet", "CronJob", "Job"] },
  { id: "pods", label: "容器组", kinds: ["Pod"] },
  { id: "networks", label: "网络", kinds: ["Service", "Ingress", "Endpoints", "NetworkPolicy"] },
  { id: "storages", label: "存储", kinds: ["PersistentVolumeClaim"] },
  { id: "configs", label: "配置", kinds: ["ConfigMap", "Secret", "HorizontalPodAutoscaler", "PodDisruptionBudget"] },
] as const;

export type EnvResourceGroupId = (typeof ENV_RESOURCE_GROUPS)[number]["id"];
export type EnvResourceKind = (typeof ENV_RESOURCE_GROUPS)[number]["kinds"][number];
