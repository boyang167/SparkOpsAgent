import type { BffContext } from "@sparkops/bff-core";

export async function getDashboard(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.platform.getDashboard(workspaceId);
}

export async function getMonitorOverview(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.platform.getMonitorOverview(workspaceId);
}

export async function listDeploymentStats(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.platform.listDeploymentStats(workspaceId);
  return data.items;
}

export async function listClusters(ctx: BffContext) {
  const data = await ctx.upstream.platform.listClusters();
  return data.items;
}

export async function createCluster(ctx: BffContext, input: Record<string, unknown>) {
  return ctx.upstream.platform.createCluster(input);
}

export async function listSyncJobs(ctx: BffContext) {
  const data = await ctx.upstream.platform.listSyncJobs();
  return data.items;
}

export async function createSyncJob(ctx: BffContext, input: Record<string, unknown>) {
  return ctx.upstream.platform.createSyncJob(input);
}

export async function getSysConfig(ctx: BffContext) {
  return ctx.upstream.platform.getSysConfig();
}

export async function updateSysConfig(ctx: BffContext, input: Record<string, unknown>) {
  return ctx.upstream.platform.updateSysConfig(input);
}

export async function getServerStatus(ctx: BffContext) {
  return ctx.upstream.platform.getServerStatus();
}

export async function listProclaims(ctx: BffContext) {
  const data = await ctx.upstream.platform.listProclaims();
  return data.items;
}

export async function createProclaim(ctx: BffContext, input: Record<string, unknown>) {
  return ctx.upstream.platform.createProclaim(input);
}

export async function listTools(ctx: BffContext) {
  const data = await ctx.upstream.platform.listTools();
  return data.items;
}
