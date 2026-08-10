import { bffFetch } from "./client";
import type {
  DeploymentStatVM,
  MonitorOverviewVM,
  ProclaimVM,
  ServerStatusVM,
  SyncJobVM,
  SysConfigVM,
  ToolVM,
} from "@sparkops/contracts/platform";

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

export async function getServerStatus() {
  return bffFetch<ServerStatusVM>("/admin/server/status");
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

export async function listSyncJobs() {
  const data = await bffFetch<{ items: SyncJobVM[] }>("/admin/sync/jobs");
  return data.items;
}

export function createSyncJob(input: Record<string, unknown>) {
  return bffFetch<{ id: string; name: string; status: string }>("/admin/sync/jobs", {
    method: "POST",
    body: JSON.stringify(input),
  });
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
