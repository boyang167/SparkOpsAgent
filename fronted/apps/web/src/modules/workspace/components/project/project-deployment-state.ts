import type { DeploymentRunListVM, DeploymentRunVM } from "@sparkops/contracts/delivery";

export function hasSuccessfulDeployment(
  page: DeploymentRunListVM | null | undefined,
): boolean {
  return page?.items.some((item) => item.status === "succeeded") ?? false;
}

export function hasDeploymentHistory(
  page: DeploymentRunListVM | null | undefined,
): boolean {
  return (page?.items.length ?? 0) > 0;
}

export function latestSuccessfulDeployment(
  page: DeploymentRunListVM | null | undefined,
): DeploymentRunVM | null {
  if (!page?.items.length) return null;
  return page.items.find((item) => item.status === "succeeded") ?? null;
}

export function succeededDeployments(
  page: DeploymentRunListVM | null | undefined,
): DeploymentRunVM[] {
  if (!page?.items.length) return [];
  return page.items.filter((item) => item.status === "succeeded" && item.image_tag.trim());
}

export function findDeploymentRun(
  page: DeploymentRunListVM | null | undefined,
  runId: string | null | undefined,
): DeploymentRunVM | null {
  if (!runId || !page?.items.length) return null;
  return page.items.find((item) => item.id === runId) ?? null;
}

export function canRollbackDeployment(
  page: DeploymentRunListVM | null | undefined,
  hasActiveRun: boolean,
): { allowed: boolean; reason: string | null } {
  if (hasActiveRun) {
    return { allowed: false, reason: "已有部署正在进行" };
  }
  if (succeededDeployments(page).length < 2) {
    return { allowed: false, reason: "至少需要两次成功发布才能回滚" };
  }
  return { allowed: true, reason: null };
}

const PUBLISHED_ONLY_TABS = new Set([
  "config",
  "build-logs",
  "storage",
  "webhooks",
  "monitor",
]);

/** 首次点击发布后切换为发布态 Tab；成功发布后保持发布态。 */
export function isPublishedProjectView(
  page: DeploymentRunListVM | null | undefined,
  tabParam?: string | null,
): boolean {
  if (hasSuccessfulDeployment(page)) return true;
  if (hasDeploymentHistory(page)) return true;
  return Boolean(tabParam && PUBLISHED_ONLY_TABS.has(tabParam));
}
