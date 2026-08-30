export type ResourceDef = {
  id: string;
  group: string;
  version: string;
  kind: string;
  plural: string;
  namespaced: boolean;
  columns: { key: string; title: string }[];
  menu: string;
  adminOnly?: boolean;
};

export type ClusterOption = { id: string };

export type DeploymentDraft = {
  namespace: string;
  name: string;
  image: string;
  replicas: number;
  ports: string;
};

export type DeploymentDraftErrors = Partial<Record<keyof DeploymentDraft, string>>;

const baseColumns = [
  { key: "metadata.name", title: "名称" },
  { key: "metadata.namespace", title: "命名空间" },
  { key: "metadata.creationTimestamp", title: "创建时间" },
];

const clusterColumns = [
  { key: "metadata.name", title: "名称" },
  { key: "metadata.creationTimestamp", title: "创建时间" },
];

export const CATALOG: ResourceDef[] = [
  def("core-nodes", "nodes", "", "v1", "Node", "nodes", false, [
    { key: "metadata.name", title: "名称" },
    { key: "status.phase", title: "状态" },
    { key: "metadata.creationTimestamp", title: "创建时间" },
  ]),
  def("core-namespaces", "namespaces", "", "v1", "Namespace", "namespaces", false, clusterColumns),
  def("core-events", "namespaces", "", "v1", "Event", "events", true, [
    { key: "metadata.namespace", title: "命名空间" },
    { key: "involvedObject.name", title: "对象" },
    { key: "reason", title: "原因" },
    { key: "message", title: "消息" },
  ]),
  def("core-pods", "workloads", "", "v1", "Pod", "pods", true, [
    { key: "metadata.name", title: "名称" },
    { key: "metadata.namespace", title: "命名空间" },
    { key: "status.phase", title: "状态" },
    { key: "metadata.creationTimestamp", title: "创建时间" },
  ]),
  def("apps-deployments", "workloads", "apps", "v1", "Deployment", "deployments", true, baseColumns),
  def("apps-daemonsets", "workloads", "apps", "v1", "DaemonSet", "daemonsets", true, baseColumns),
  def("apps-statefulsets", "workloads", "apps", "v1", "StatefulSet", "statefulsets", true, baseColumns),
  def("apps-replicasets", "workloads", "apps", "v1", "ReplicaSet", "replicasets", true, baseColumns),
  def("batch-jobs", "workloads", "batch", "v1", "Job", "jobs", true, baseColumns),
  def("batch-cronjobs", "workloads", "batch", "v1", "CronJob", "cronjobs", true, baseColumns),
  def("core-services", "networks", "", "v1", "Service", "services", true, [
    { key: "metadata.name", title: "名称" },
    { key: "metadata.namespace", title: "命名空间" },
    { key: "spec.type", title: "类型" },
    { key: "metadata.creationTimestamp", title: "创建时间" },
  ]),
  def("core-endpoints", "networks", "", "v1", "Endpoints", "endpoints", true, baseColumns),
  def("networking-ingresses", "networks", "networking.k8s.io", "v1", "Ingress", "ingresses", true, baseColumns),
  def("networking-networkpolicies", "networks", "networking.k8s.io", "v1", "NetworkPolicy", "networkpolicies", true, baseColumns),
  def("gateway-classes", "gateway", "gateway.networking.k8s.io", "v1", "GatewayClass", "gatewayclasses", false, clusterColumns),
  def("gateway-gateways", "gateway", "gateway.networking.k8s.io", "v1", "Gateway", "gateways", true, baseColumns),
  def("gateway-httproutes", "gateway", "gateway.networking.k8s.io", "v1", "HTTPRoute", "httproutes", true, baseColumns),
  def("gateway-grpcroutes", "gateway", "gateway.networking.k8s.io", "v1", "GRPCRoute", "grpcroutes", true, baseColumns),
  def("gateway-tcproutes", "gateway", "gateway.networking.k8s.io", "v1alpha2", "TCPRoute", "tcproutes", true, baseColumns),
  def("gateway-tlsroutes", "gateway", "gateway.networking.k8s.io", "v1alpha2", "TLSRoute", "tlsroutes", true, baseColumns),
  def("gateway-udproutes", "gateway", "gateway.networking.k8s.io", "v1alpha2", "UDPRoute", "udproutes", true, baseColumns),
  def("core-configmaps", "configs", "", "v1", "ConfigMap", "configmaps", true, baseColumns),
  def("core-secrets", "configs", "", "v1", "Secret", "secrets", true, baseColumns, true),
  def("core-persistentvolumes", "storages", "", "v1", "PersistentVolume", "persistentvolumes", false, clusterColumns),
  def("core-persistentvolumeclaims", "storages", "", "v1", "PersistentVolumeClaim", "persistentvolumeclaims", true, baseColumns),
  def("core-serviceaccounts", "access-controls", "", "v1", "ServiceAccount", "serviceaccounts", true, baseColumns),
  def("rbac-roles", "access-controls", "rbac.authorization.k8s.io", "v1", "Role", "roles", true, baseColumns),
  def("rbac-rolebindings", "access-controls", "rbac.authorization.k8s.io", "v1", "RoleBinding", "rolebindings", true, baseColumns),
  def("rbac-clusterroles", "access-controls", "rbac.authorization.k8s.io", "v1", "ClusterRole", "clusterroles", false, clusterColumns),
  def("rbac-clusterrolebindings", "access-controls", "rbac.authorization.k8s.io", "v1", "ClusterRoleBinding", "clusterrolebindings", false, clusterColumns),
  def("policy-podsecuritypolicies", "access-controls", "policy", "v1beta1", "PodSecurityPolicy", "podsecuritypolicies", false, clusterColumns),
  def("kruise-clonesets", "openkruise", "apps.kruise.io", "v1alpha1", "CloneSet", "clonesets", true, baseColumns),
  def("kruise-advancedcronjobs", "openkruise", "apps.kruise.io", "v1alpha1", "AdvancedCronJob", "advancedcronjobs", true, baseColumns),
  def("kruise-sidecarsets", "openkruise", "apps.kruise.io", "v1alpha1", "SidecarSet", "sidecarsets", false, clusterColumns),
  def("kruise-broadcastjobs", "openkruise", "apps.kruise.io", "v1alpha1", "BroadcastJob", "broadcastjobs", true, baseColumns),
  def("rollouts-rollouts", "kruise-rollouts", "rollouts.kruise.io", "v1alpha1", "Rollout", "rollouts", true, baseColumns),
  def("rollouts-batchreleases", "kruise-rollouts", "rollouts.kruise.io", "v1alpha1", "BatchRelease", "batchreleases", true, baseColumns),
  def("rollouts-trafficroutings", "kruise-rollouts", "rollouts.kruise.io", "v1alpha1", "TrafficRouting", "trafficroutings", true, baseColumns),
  def("kubeblocks-clusters", "kubeblocks", "apps.kubeblocks.io", "v1alpha1", "Cluster", "clusters", true, baseColumns),
  def("kubeblocks-componentdefinitions", "kubeblocks", "apps.kubeblocks.io", "v1alpha1", "ComponentDefinition", "componentdefinitions", false, clusterColumns),
  def("kubevela-applications", "kubevela", "core.oam.dev", "v1beta1", "Application", "applications", true, baseColumns),
  def("velero-backups", "backup", "velero.io", "v1", "Backup", "backups", true, baseColumns),
  def("velero-restores", "backup", "velero.io", "v1", "Restore", "restores", true, baseColumns),
  def("velero-schedules", "backup", "velero.io", "v1", "Schedule", "schedules", true, baseColumns),
];

export function resourcesForMenu(menu: string, isAdmin = false): ResourceDef[] {
  return CATALOG.filter((item) => item.menu === menu && (!item.adminOnly || isAdmin));
}

export function proxyPath(clusterId: string, def: ResourceDef, ns?: string, name?: string): string {
  const parts = [
    "/api/bff/v1/plugins/kubernetes/clusters",
    encodeSegment(clusterId),
    "proxy",
    def.group ? "apis" : "api",
  ];
  if (def.group) {
    parts.push(encodeSegment(def.group));
  }
  parts.push(encodeSegment(def.version));
  if (def.namespaced && ns) {
    parts.push("namespaces", encodeSegment(ns));
  }
  parts.push(encodeSegment(def.plural));
  if (name) {
    parts.push(encodeSegment(name));
  }
  return parts.join("/");
}

export function serviceProxyPath(clusterId: string, namespace: string, serviceSegment: string): string {
  const match = /^(https?):([^:]+):([0-9]+)$/.exec(serviceSegment);
  const name = match?.[2] ?? "";
  const port = Number(match?.[3]);
  if (
    !match ||
    serviceSegment.includes("%") ||
    serviceSegment.includes("/") ||
    serviceSegment.includes("\\") ||
    serviceSegment.includes("@") ||
    !isDNSSubdomain(name) ||
    !Number.isInteger(port) ||
    port < 1 ||
    port > 65535
  ) {
    throw new Error("invalid kubernetes service proxy segment");
  }
  const normalized = `${match[1]}:${name}:${port}`;
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeSegment(clusterId)}` +
    `/proxy/api/v1/namespaces/${encodeSegment(namespace)}/services/${normalized}/proxy/`;
}

export function selectStoredClusterId(stored: string | null, clusters: ClusterOption[]): string {
  if (clusters.length === 0) return "";
  if (stored && clusters.some((item) => item.id === stored)) return stored;
  return clusters[0]?.id ?? "";
}

export function validateDeploymentDraft(draft: DeploymentDraft): DeploymentDraftErrors {
  const errors: DeploymentDraftErrors = {};
  if (!isDNSLabel(draft.namespace)) errors.namespace = "命名空间必须是 DNS label";
  if (!isDNSSubdomain(draft.name)) errors.name = "名称必须是 DNS subdomain";
  if (!draft.image.trim() || /\s/.test(draft.image)) errors.image = "镜像不能为空";
  if (!Number.isInteger(draft.replicas) || draft.replicas < 1 || draft.replicas > 1000) {
    errors.replicas = "副本数必须在 1-1000 之间";
  }
  if (parsePorts(draft.ports) === null) errors.ports = "端口必须是 1-65535 的数字";
  return errors;
}

export function buildDeploymentManifest(draft: DeploymentDraft) {
  const ports = parsePorts(draft.ports) ?? [];
  return {
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: {
      name: draft.name.trim(),
      namespace: draft.namespace.trim(),
    },
    spec: {
      replicas: draft.replicas,
      selector: { matchLabels: { app: draft.name.trim() } },
      template: {
        metadata: { labels: { app: draft.name.trim() } },
        spec: {
          containers: [
            {
              name: draft.name.trim(),
              image: draft.image.trim(),
              ports: ports.map((containerPort) => ({ containerPort })),
            },
          ],
        },
      },
    },
  };
}

function def(
  id: string,
  menu: string,
  group: string,
  version: string,
  kind: string,
  plural: string,
  namespaced: boolean,
  columns: ResourceDef["columns"],
  adminOnly = false,
): ResourceDef {
  return { id, menu, group, version, kind, plural, namespaced, columns, adminOnly };
}

function encodeSegment(value: string): string {
  return encodeURIComponent(normalizePathSegment(value));
}

function normalizePathSegment(value: string): string {
  let current = value;
  for (let depth = 0; depth < 3; depth += 1) {
    assertPathSegment(current);
    let decoded: string;
    try {
      decoded = decodeURIComponent(current);
    } catch {
      throw new Error("invalid kubernetes path segment");
    }
    if (decoded === current) {
      return current;
    }
    current = decoded;
  }

  assertPathSegment(current);
  if (/%[0-9a-fA-F]{2}/.test(current)) {
    throw new Error("invalid kubernetes path segment");
  }
  return current;
}

function assertPathSegment(value: string) {
  if (
    !value ||
    value === "." ||
    value === ".." ||
    value.includes("/") ||
    value.includes("\\") ||
    value.includes("\u0000") ||
    value.includes("://") ||
    /^[a-z][a-z0-9+.-]*:/i.test(value)
  ) {
    throw new Error("invalid kubernetes path segment");
  }
}

function isDNSLabel(value: string): boolean {
  return /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/.test(value.trim()) && value.trim().length <= 63;
}

function isDNSSubdomain(value: string): boolean {
  const trimmed = value.trim();
  return (
    trimmed.length > 0 &&
    trimmed.length <= 253 &&
    trimmed.split(".").every((part) => isDNSLabel(part))
  );
}

function parsePorts(value: string): number[] | null {
  const trimmed = value.trim();
  if (!trimmed) return [];
  const ports = trimmed.split(",").map((part) => Number(part.trim()));
  if (ports.some((port) => !Number.isInteger(port) || port < 1 || port > 65535)) return null;
  return ports;
}
