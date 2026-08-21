import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";

export type EntitlementModuleKey = keyof WorkspaceEntitlementsVM;

/** Convert BFF route path to upstream API path for entitlement lookup. */
export function bffPathToApiPath(bffPath: string): string {
  if (bffPath.startsWith("/api/bff/v1")) {
    return bffPath.replace(/^\/api\/bff\/v1/, "/api/v1");
  }
  if (bffPath.startsWith("/api/v1")) {
    return bffPath;
  }
  return `/api/v1${bffPath.startsWith("/") ? bffPath : `/${bffPath}`}`;
}

function shouldSkipApiEntitlement(path: string): boolean {
  const prefixes = [
    "/api/v1/admin/",
    "/api/v1/auth/",
    "/api/v1/public/",
    "/api/v1/platform/",
    "/api/v1/orgs/",
    "/api/v1/notifications",
    "/api/v1/models",
    "/api/v1/identity/",
    "/api/v1/members/",
  ];
  if (prefixes.some((prefix) => path.startsWith(prefix))) {
    return true;
  }
  if (path.endsWith("/entitlements") || path.endsWith("/dashboard")) {
    return true;
  }
  if (path === "/api/v1/workspaces" || path.endsWith("/workspaces/select")) {
    return true;
  }
  return false;
}

function isGovernanceWorkspaceApi(path: string): boolean {
  if (!path.includes("/workspaces/")) return false;
  const markers = [
    "/roles",
    "/members",
    "/me/",
    "/menus",
    "/apis",
    "/api-tokens",
    "/data-resources",
    "/operation-records",
  ];
  return markers.some((marker) => path.includes(marker));
}

/** Mirrors backend entitlements.APIPathEntitlement. */
export function apiPathEntitlement(path: string): EntitlementModuleKey | "" {
  if (!path.startsWith("/api/v1/")) return "";
  if (shouldSkipApiEntitlement(path)) return "";
  if (isGovernanceWorkspaceApi(path)) return "";

  if (path.includes("/knowledge-bases") || path.startsWith("/api/v1/documents/")) {
    return "knowledge";
  }
  if (path.includes("/market/")) return "marketplace";
  if (path.includes("/builds") || path.includes("/products")) return "delivery";
  if (path.includes("/webhooks") || path.includes("/mcp/")) return "mcp";
  if (
    path.includes("/cronjobs") ||
    path.includes("/pods") ||
    path.includes("/k8s/") ||
    path.includes("/secrets") ||
    path.includes("/consul/") ||
    path.includes("/deployments") ||
    path.includes("/ingresses") ||
    path.includes("/configmaps") ||
    path.includes("/services") ||
    path.includes("/pvcs") ||
    path.includes("/virtualservices") ||
    path.includes("/storageclasses")
  ) {
    return "k8s";
  }
  if (
    path.includes("/monitor/") ||
    path.includes("/observability/") ||
    path.includes("/statistics")
  ) {
    return "observability";
  }
  if (
    path.includes("/projects") ||
    path.startsWith("/api/v1/agents/") ||
    path.startsWith("/api/v1/runs/") ||
    path.startsWith("/api/v1/conversations/") ||
    path.startsWith("/api/v1/workflows/") ||
    path.startsWith("/api/v1/workflow-runs/") ||
    path.includes("/templates")
  ) {
    return "agent";
  }
  return "";
}

export function bffPathEntitlement(bffPath: string): EntitlementModuleKey | "" {
  return apiPathEntitlement(bffPathToApiPath(bffPath));
}

export function isModuleEnabled(
  entitlements: WorkspaceEntitlementsVM,
  moduleKey: EntitlementModuleKey | "",
): boolean {
  if (!moduleKey) return true;
  return entitlements[moduleKey];
}

export const DEFAULT_ENTITLEMENTS: WorkspaceEntitlementsVM = {
  agent: true,
  knowledge: true,
  marketplace: true,
  delivery: true,
  observability: true,
  k8s: false,
  mcp: false,
};
