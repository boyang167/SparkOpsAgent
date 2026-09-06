import type { DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";

export function hasSuccessfulDeployment(
  page: DeploymentRunListVM | null | undefined,
): boolean {
  return page?.items.some((item) => item.status === "succeeded") ?? false;
}

export function latestSuccessfulDeployment(
  page: DeploymentRunListVM | null | undefined,
): DeploymentRunVM | null {
  if (!page?.items.length) return null;
  return page.items.find((item) => item.status === "succeeded") ?? null;
}

const PUBLISHED_ONLY_TABS = new Set([
  "config",
  "build-logs",
  "storage",
  "webhooks",
  "monitor",
]);

export function isPublishedProjectView(
  page: DeploymentRunListVM | null | undefined,
  tabParam?: string | null,
): boolean {
  if (hasSuccessfulDeployment(page)) return true;
  return Boolean(tabParam && PUBLISHED_ONLY_TABS.has(tabParam));
}
