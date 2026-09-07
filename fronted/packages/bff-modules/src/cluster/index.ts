import type { BffContext } from "@sparkops/bff-core";
import type { PodVM, ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";

export async function listPods(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<PodVM[]> {
  const data = await ctx.upstream.cluster.listPods(workspaceId, projectId);
  return data.items;
}

export async function getProjectRuntimeOverview(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<ProjectRuntimeOverviewVM> {
  return ctx.upstream.cluster.getProjectRuntimeOverview(workspaceId, projectId);
}

export async function getPod(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  podId: string,
): Promise<PodVM> {
  return ctx.upstream.cluster.getPod(workspaceId, projectId, podId);
}

export async function getPodLogs(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  podId: string,
): Promise<{ logs: string }> {
  return ctx.upstream.cluster.getPodLogs(workspaceId, projectId, podId);
}

export async function restartPod(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  podId: string,
): Promise<{ restarted: boolean }> {
  return ctx.upstream.cluster.restartPod(workspaceId, projectId, podId);
}
