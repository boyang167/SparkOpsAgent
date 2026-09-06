import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";

export const DEFAULT_ENTITLEMENTS: WorkspaceEntitlementsVM = {
  agent: true,
  knowledge: true,
  delivery: true,
  observability: true,
  k8s: false,
  mcp: false,
  plugins: [],
};

export type EntitlementKey = string;

export function pathEntitlement(path: string): string {
  if (!path.startsWith("/")) return "";
  if (path.startsWith("/plugins/")) {
    const pluginId = path.slice("/plugins/".length).split("/")[0];
    return pluginId ? `plugin:${pluginId}` : "";
  }
  if (path.startsWith("/knowledge")) return "knowledge";
  if (path.startsWith("/delivery") || path.startsWith("/publish") || path.startsWith("/config/secrets")) {
    return "delivery";
  }
  if (path.startsWith("/monitor")) return "observability";
  if (path.startsWith("/config/webhooks") || path.startsWith("/config/mcp")) return "mcp";
  if (
    path.startsWith("/runtime") ||
    path.startsWith("/cronjobs")
  ) {
    return "k8s";
  }
  if (path.startsWith("/projects") || path.startsWith("/templates")) return "agent";
  return "";
}

export function isPathEntitled(path: string, entitlements: WorkspaceEntitlementsVM): boolean {
  const key = pathEntitlement(path);
  if (!key) return true;
  if (key.startsWith("plugin:")) {
    return entitlements.plugins.includes(key.slice("plugin:".length));
  }
  if (key in entitlements) {
    const value = entitlements[key as keyof WorkspaceEntitlementsVM];
    return typeof value === "boolean" ? value : true;
  }
  return true;
}

export function filterNavItemsByEntitlements<T extends { href: string }>(
  items: T[],
  entitlements: WorkspaceEntitlementsVM,
): T[] {
  return items.filter((item) => isPathEntitled(item.href, entitlements));
}

export function filterNavGroupsByEntitlements<
  T extends { items: Array<{ href: string }> },
>(groups: T[], entitlements: WorkspaceEntitlementsVM): T[] {
  return groups
    .map((group) => ({
      ...group,
      items: filterNavItemsByEntitlements(group.items, entitlements),
    }))
    .filter((group) => group.items.length > 0);
}
