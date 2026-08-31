export type ClusterReachability = "checking" | "healthy" | "unhealthy";

export type ClusterStatusView = {
  kind: ClusterReachability;
  label: string;
  detail?: string;
};

export function clusterVersionProxyPath(clusterId: string): string {
  return `/api/bff/v1/plugins/kubernetes/clusters/${encodeURIComponent(clusterId)}/proxy/version`;
}

export function clusterStatusView(kind: ClusterReachability, detail?: string): ClusterStatusView {
  if (kind === "checking") return { kind, label: "检测中" };
  if (kind === "healthy") return { kind, label: "正常" };
  return { kind, label: "异常", ...(detail ? { detail } : {}) };
}

export function clusterStatusFromProbe(ok: boolean, errorMessage?: string): ClusterStatusView {
  return ok ? clusterStatusView("healthy") : clusterStatusView("unhealthy", errorMessage);
}

export function clusterStatusBadgeClass(kind: ClusterReachability): string {
  if (kind === "healthy") return "badge badge-success";
  if (kind === "unhealthy") return "badge badge-danger";
  return "badge badge-neutral";
}
