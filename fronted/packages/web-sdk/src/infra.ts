import { bffFetch } from "./client";
import type { CronJobVM } from "@sparkops/contracts/delivery";

export async function listCronJobs(workspaceId: string, projectId: string) {
  const data = await bffFetch<{ items: CronJobVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/cronjobs`,
    { workspaceId },
  );
  return data.items;
}

export function createCronJob(
  workspaceId: string,
  projectId: string,
  input: { name: string; schedule: string; branch?: string },
) {
  return bffFetch<CronJobVM>(`/workspaces/${workspaceId}/projects/${projectId}/cronjobs`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function deleteCronJob(workspaceId: string, projectId: string, cronJobId: string) {
  return bffFetch<{ deleted: true }>(
    `/workspaces/${workspaceId}/projects/${projectId}/cronjobs/${cronJobId}`,
    {
      method: "DELETE",
      workspaceId,
    },
  );
}

export async function listK8sResources(
  workspaceId: string,
  projectId: string,
  kind: "configmaps" | "ingresses" | "services" | "pvcs" | "virtualservices",
) {
  return bffFetch<{ items: Record<string, unknown>[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/k8s/${kind}`,
    { workspaceId },
  );
}

export async function listConsulKV(workspaceId: string, prefix?: string) {
  const qs = prefix ? `?prefix=${encodeURIComponent(prefix)}` : "";
  const data = await bffFetch<{ items: { key: string; value: string; updated_at?: string }[] }>(
    `/workspaces/${workspaceId}/consul/kv${qs}`,
    { workspaceId },
  );
  return data.items;
}

export function putConsulKV(workspaceId: string, input: { key: string; value: string }) {
  return bffFetch<{ saved: boolean }>(`/workspaces/${workspaceId}/consul/kv`, {
    method: "PUT",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function deleteConsulKV(workspaceId: string, key: string) {
  return bffFetch<{ deleted: boolean }>(
    `/workspaces/${workspaceId}/consul/kv?key=${encodeURIComponent(key)}`,
    { method: "DELETE", workspaceId },
  );
}
