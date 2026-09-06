import { bffFetch } from "./client";
import type {
  DeploymentStatVM,
  MonitorOverviewVM,
  ProclaimVM,
  ServerStatusVM,
  SysConfigVM,
  ToolVM,
} from "@sparkops/contracts/platform";
import type { AuthorizedClusterVM } from "@sparkops/contracts/delivery";

export async function getMonitorOverview(workspaceId: string) {
  return bffFetch<MonitorOverviewVM>(`/workspaces/${workspaceId}/monitor/overview`, {
    workspaceId,
  });
}

export async function listDeploymentStats(workspaceId: string) {
  const data = await bffFetch<{ items: DeploymentStatVM[] }>(
    `/workspaces/${workspaceId}/statistics/deployments`,
    { workspaceId },
  );
  return data.items;
}

export function getServerStatus(workspaceId: string) {
  return bffFetch<ServerStatusVM>(`/workspaces/${workspaceId}/monitor/server/status`, {
    workspaceId,
  });
}

export function listOperationRecords(
  workspaceId: string,
  query: import("@sparkops/contracts/platform").ListOperationRecordsQuery = {},
) {
  const params = new URLSearchParams();
  if (query.page) params.set("page", String(query.page));
  if (query.page_size) params.set("page_size", String(query.page_size));
  if (query.ip) params.set("ip", query.ip);
  if (query.method) params.set("method", query.method);
  if (query.path) params.set("path", query.path);
  if (query.status != null) params.set("status", String(query.status));
  if (query.member_id) params.set("member_id", query.member_id);
  if (query.trace_id) params.set("trace_id", query.trace_id);
  const qs = params.toString();
  return bffFetch<import("@sparkops/contracts/platform").OperationRecordListVM>(
    `/workspaces/${workspaceId}/operation-records${qs ? `?${qs}` : ""}`,
    { workspaceId },
  );
}

export function deleteOperationRecord(workspaceId: string, recordId: string) {
  return bffFetch<{ deleted: boolean }>(
    `/workspaces/${workspaceId}/operation-records/${recordId}`,
    { method: "DELETE", workspaceId },
  );
}

export function batchDeleteOperationRecords(
  workspaceId: string,
  input: import("@sparkops/contracts/platform").BatchDeleteOperationRecordsCommand,
) {
  return bffFetch<{ deleted: number }>(
    `/workspaces/${workspaceId}/operation-records/batch-delete`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function listAppLogs(
  workspaceId: string,
  query: import("@sparkops/contracts/platform").ListAppLogsQuery = {},
) {
  const params = new URLSearchParams();
  if (query.page) params.set("page", String(query.page));
  if (query.page_size) params.set("page_size", String(query.page_size));
  if (query.level) params.set("level", query.level);
  if (query.category) params.set("category", query.category);
  if (query.trace_id) params.set("trace_id", query.trace_id);
  if (query.keyword) params.set("keyword", query.keyword);
  const qs = params.toString();
  return bffFetch<import("@sparkops/contracts/platform").AppLogListVM>(
    `/workspaces/${workspaceId}/observability/logs${qs ? `?${qs}` : ""}`,
    { workspaceId },
  );
}

export function listLogFiles(workspaceId: string) {
  return bffFetch<import("@sparkops/contracts/platform").LogFileListVM>(
    `/workspaces/${workspaceId}/observability/log-files`,
    { workspaceId },
  );
}

export function tailLogFile(
  workspaceId: string,
  fileName: string,
  offset = 0,
  limit = 200,
) {
  const params = new URLSearchParams();
  params.set("offset", String(offset));
  params.set("limit", String(limit));
  return bffFetch<import("@sparkops/contracts/platform").LogTailVM>(
    `/workspaces/${workspaceId}/observability/log-files/${encodeURIComponent(fileName)}/tail?${params}`,
    { workspaceId },
  );
}

export function getTraceDetail(workspaceId: string, traceId: string) {
  return bffFetch<import("@sparkops/contracts/platform").TraceDetailVM>(
    `/workspaces/${workspaceId}/observability/traces/${encodeURIComponent(traceId)}`,
    { workspaceId },
  );
}

export function getMetricsSummary(workspaceId: string) {
  return bffFetch<import("@sparkops/contracts/platform").MetricsSummaryVM>(
    `/workspaces/${workspaceId}/observability/metrics/summary`,
    { workspaceId },
  );
}

export function getWorkspaceEntitlements(workspaceId: string) {
  return bffFetch<import("@sparkops/contracts/platform").WorkspaceEntitlementsVM>(
    `/workspaces/${workspaceId}/entitlements`,
    { workspaceId },
  );
}

export async function listClusters() {
  const data = await bffFetch<{ items: Record<string, unknown>[] }>("/admin/clusters");
  return data.items;
}

export function createCluster(input: Record<string, unknown>) {
  return bffFetch<{ id: string; cluster_name: string }>("/admin/clusters", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function listWorkspaceClusters(workspaceId: string) {
  const data = await bffFetch<{ items: AuthorizedClusterVM[] }>(
    `/admin/workspaces/${workspaceId}/clusters`,
  );
  return data.items;
}

export async function listAuthorizedWorkspaceClusters(workspaceId: string) {
  const data = await bffFetch<{ items: AuthorizedClusterVM[] }>(
    `/workspaces/${workspaceId}/clusters`,
    { workspaceId },
  );
  return data.items;
}

export function registerWorkspaceCluster(
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
  return bffFetch<{ id: string; cluster_name: string; server?: string }>(
    `/workspaces/${workspaceId}/clusters`,
    {
      method: "POST",
      workspaceId,
      body: JSON.stringify(input),
    },
  );
}

export function revokeAuthorizedWorkspaceCluster(workspaceId: string, clusterId: string) {
  return bffFetch<{ revoked: true }>(`/workspaces/${workspaceId}/clusters/${clusterId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export function reorderAuthorizedWorkspaceClusters(workspaceId: string, clusterIds: string[]) {
  return bffFetch<{ reordered: true }>(`/workspaces/${workspaceId}/clusters/order`, {
    method: "PUT",
    workspaceId,
    body: JSON.stringify({ cluster_ids: clusterIds }),
  });
}

export function grantWorkspaceCluster(workspaceId: string, clusterId: string) {
  return bffFetch<{ granted: true }>(`/admin/workspaces/${workspaceId}/clusters`, {
    method: "POST",
    body: JSON.stringify({ cluster_id: clusterId }),
  });
}

export function revokeWorkspaceCluster(workspaceId: string, clusterId: string) {
  return bffFetch<{ revoked: true }>(
    `/admin/workspaces/${workspaceId}/clusters/${clusterId}`,
    { method: "DELETE" },
  );
}

export async function getSysConfig() {
  return bffFetch<SysConfigVM>("/admin/sysconfig");
}

export function updateSysConfig(input: Record<string, unknown>) {
  return bffFetch<SysConfigVM>("/admin/sysconfig", {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export async function listProclaims() {
  const data = await bffFetch<{ items: ProclaimVM[] }>("/admin/proclaims");
  return data.items;
}

export function createProclaim(input: Record<string, unknown>) {
  return bffFetch<{ id: string; title: string }>("/admin/proclaims", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export async function listTools() {
  const data = await bffFetch<{ items: ToolVM[] }>("/admin/tools");
  return data.items;
}
