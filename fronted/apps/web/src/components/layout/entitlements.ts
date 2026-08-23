import type { WorkspaceEntitlementsVM } from "@sparkops/contracts/platform";

export const DEFAULT_ENTITLEMENTS: WorkspaceEntitlementsVM = {
  agent: true,
  knowledge: true,
  marketplace: true,
  delivery: true,
  observability: true,
  k8s: false,
  mcp: false,
};

export type EntitlementKey = keyof WorkspaceEntitlementsVM;

export function pathEntitlement(path: string): EntitlementKey | "" {
  if (!path.startsWith("/")) return "";
  if (path.startsWith("/knowledge")) return "knowledge";
  if (path.startsWith("/market")) return "marketplace";
  if (path.startsWith("/delivery") || path.startsWith("/publish")) return "delivery";
  if (path.startsWith("/monitor")) return "observability";
  if (path.startsWith("/config/webhooks") || path.startsWith("/config/mcp")) return "mcp";
  if (
    path.startsWith("/runtime") ||
    path.startsWith("/cronjobs") ||
    path.startsWith("/config") ||
    path.startsWith("/discovery") ||
    path.startsWith("/security")
  ) {
    return "k8s";
  }
  if (path.startsWith("/projects")) return "agent";
  return "";
}

export function isPathEntitled(path: string, entitlements: WorkspaceEntitlementsVM): boolean {
  const key = pathEntitlement(path);
  if (!key) return true;
  return entitlements[key];
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
