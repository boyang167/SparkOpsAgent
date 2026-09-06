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
