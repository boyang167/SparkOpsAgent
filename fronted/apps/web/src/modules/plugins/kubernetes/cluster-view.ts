import type { KubernetesCluster } from "./cluster-types";

export type ClusterIntegration = "prometheus" | "velaux" | "llm";

const INTEGRATION_LABELS: Record<ClusterIntegration, string> = {
  prometheus: "Prometheus",
  velaux: "Velaux",
  llm: "LLM",
};

export function clusterIntegrations(cluster: KubernetesCluster): ClusterIntegration[] {
  const items: ClusterIntegration[] = [];
  if (cluster.prometheus_url?.trim()) items.push("prometheus");
  if (cluster.velaux_url?.trim()) items.push("velaux");
  if (cluster.llm_base_url?.trim()) items.push("llm");
  return items;
}

export function clusterIntegrationLabel(key: ClusterIntegration): string {
  return INTEGRATION_LABELS[key];
}

export function truncateClusterId(id: string, length = 8): string {
  const trimmed = id.trim();
  if (trimmed.length <= length) return trimmed;
  return `${trimmed.slice(0, length)}…`;
}

export function formatClusterServer(server: string | undefined): string {
  const trimmed = (server ?? "").trim();
  return trimmed || "未配置接口地址";
}
