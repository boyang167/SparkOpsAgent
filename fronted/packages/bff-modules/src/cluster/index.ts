import type { BffContext } from "@sparkops/bff-core";
import type { PodVM } from "@sparkops/contracts/cluster";

export async function listPods(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<PodVM[]> {
  const data = await ctx.upstream.cluster.listPods(workspaceId, projectId);
  return data.items;
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

export async function listConfigMaps(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
) {
  return ctx.upstream.cluster.listConfigMaps(workspaceId, projectId);
}

export async function listIngresses(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
) {
  return ctx.upstream.cluster.listIngresses(workspaceId, projectId);
}

export async function listServices(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
) {
  return ctx.upstream.cluster.listServices(workspaceId, projectId);
}

export async function listPVCs(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
) {
  return ctx.upstream.cluster.listPVCs(workspaceId, projectId);
}

export async function listVirtualServices(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
) {
  return ctx.upstream.cluster.listVirtualServices(workspaceId, projectId);
}

export async function listConsulKV(ctx: BffContext, workspaceId: string, prefix?: string) {
  return ctx.upstream.cluster.listConsulKV(workspaceId, prefix);
}

export async function putConsulKV(
  ctx: BffContext,
  workspaceId: string,
  input: { key: string; value: string },
) {
  return ctx.upstream.cluster.putConsulKV(workspaceId, input);
}

export async function deleteConsulKV(ctx: BffContext, workspaceId: string, key: string) {
  return ctx.upstream.cluster.deleteConsulKV(workspaceId, key);
}
