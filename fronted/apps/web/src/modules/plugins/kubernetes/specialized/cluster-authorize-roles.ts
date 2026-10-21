type RoleObject = {
  metadata?: {
    name?: string;
    labels?: Record<string, string>;
  };
};

const BUILTIN_CLUSTER_ROLES = new Set(["cluster-admin", "admin", "edit", "view"]);

export function isBuiltinRole(item: RoleObject, scope: "cluster" | "namespace"): boolean {
  const name = item.metadata?.name ?? "";
  if (!name) return false;
  if (name.startsWith("system:") || name.startsWith("kubeadm:")) return true;
  if (item.metadata?.labels?.["kubernetes.io/bootstrapping"] === "rbac-defaults") return true;
  if (scope === "cluster" && BUILTIN_CLUSTER_ROLES.has(name)) return true;
  return false;
}

export function formatRoleAge(timestamp?: string, now = Date.now()): string {
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
  if (days < 30) return `${days} 天`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months} 个月`;
  return `${Math.floor(days / 365)} 年`;
}
