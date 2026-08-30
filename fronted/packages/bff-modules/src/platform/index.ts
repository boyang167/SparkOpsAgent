import type { BffContext } from "@sparkops/bff-core";
import { invalidateEntitlementsCache, resolveWorkspaceEntitlements } from "@sparkops/bff-core";

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
  const result = await ctx.upstream.platform.createCluster(input);
  invalidateEntitlementsCache();
  return result;
}

export async function listWorkspaceClusters(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.platform.listWorkspaceClusters(workspaceId);
  return data.items;
}

export async function listAuthorizedWorkspaceClusters(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.platform.listAuthorizedWorkspaceClusters(workspaceId);
  return data.items;
}

export async function registerWorkspaceCluster(
  ctx: BffContext,
  workspaceId: string,
  input: {
    cluster_name: string;
    server?: string;
    api_address?: string;
    kube_type?: number;
    kube_config?: string;
    bearer_token?: string;
    user_name?: string;
    certificate_authority_data?: string;
    client_certificate_data?: string;
    client_key_data?: string;
  },
) {
  const result = await ctx.upstream.platform.registerWorkspaceCluster(workspaceId, input);
  invalidateEntitlementsCache();
  return result;
}

export async function revokeAuthorizedWorkspaceCluster(
  ctx: BffContext,
  workspaceId: string,
  clusterId: string,
) {
  return ctx.upstream.platform.revokeAuthorizedWorkspaceCluster(workspaceId, clusterId);
}

export async function grantWorkspaceCluster(
  ctx: BffContext,
  workspaceId: string,
  clusterId: string,
) {
  return ctx.upstream.platform.grantWorkspaceCluster(workspaceId, clusterId);
}

export async function revokeWorkspaceCluster(
  ctx: BffContext,
  workspaceId: string,
  clusterId: string,
) {
  return ctx.upstream.platform.revokeWorkspaceCluster(workspaceId, clusterId);
}

export async function getSysConfig(ctx: BffContext) {
  return ctx.upstream.platform.getSysConfig();
}

export async function updateSysConfig(ctx: BffContext, input: Record<string, unknown>) {
  const result = await ctx.upstream.platform.updateSysConfig(input);
  invalidateEntitlementsCache();
  return result;
}

export async function getServerStatus(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.platform.getWorkspaceServerStatus(workspaceId);
}

export async function getAdminServerStatus(ctx: BffContext) {
  return ctx.upstream.platform.getServerStatus();
}

export async function listOperationRecords(
  ctx: BffContext,
  workspaceId: string,
  query: import("@sparkops/contracts/platform").ListOperationRecordsQuery,
) {
  return ctx.upstream.platform.listOperationRecords(workspaceId, query);
}

export async function deleteOperationRecord(
  ctx: BffContext,
  workspaceId: string,
  recordId: string,
) {
  return ctx.upstream.platform.deleteOperationRecord(workspaceId, recordId);
}

export async function batchDeleteOperationRecords(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/platform").BatchDeleteOperationRecordsCommand,
) {
  return ctx.upstream.platform.batchDeleteOperationRecords(workspaceId, input);
}

export async function listAppLogs(
  ctx: BffContext,
  workspaceId: string,
  query: import("@sparkops/contracts/platform").ListAppLogsQuery,
) {
  return ctx.upstream.platform.listAppLogs(workspaceId, query);
}

export async function listLogFiles(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.platform.listLogFiles(workspaceId);
}

export async function tailLogFile(
  ctx: BffContext,
  workspaceId: string,
  fileName: string,
  offset = 0,
  limit = 200,
) {
  return ctx.upstream.platform.tailLogFile(workspaceId, fileName, offset, limit);
}

export async function getTraceDetail(ctx: BffContext, workspaceId: string, traceId: string) {
  return ctx.upstream.platform.getTraceDetail(workspaceId, traceId);
}

export async function getMetricsSummary(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.platform.getMetricsSummary(workspaceId);
}

export async function getWorkspaceEntitlements(ctx: BffContext, workspaceId: string) {
  return resolveWorkspaceEntitlements(
    workspaceId,
    (wsId) => ctx.upstream.platform.getWorkspaceEntitlements(wsId),
    { fresh: true },
  );
}

export async function getWorkspaceEntitlementsCached(ctx: BffContext, workspaceId: string) {
  return resolveWorkspaceEntitlements(workspaceId, (wsId) =>
    ctx.upstream.platform.getWorkspaceEntitlements(wsId),
  );
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
