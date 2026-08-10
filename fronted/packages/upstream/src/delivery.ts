import type { BuildVM, CronJobVM } from "@sparkops/contracts/delivery";
import type { UpstreamRequestOptions } from "./core";
import { getUpstreamBaseUrl, upstreamFetch } from "./core";

export function createDeliveryUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listBuilds(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: BuildVM[] }>(`/api/v1/projects/${projectId}/builds`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async triggerBuild(workspaceId: string, projectId: string, branch?: string) {
      return upstreamFetch<BuildVM>(`/api/v1/projects/${projectId}/builds`, {
        method: "POST",
        body: JSON.stringify({ branch: branch ?? "main" }),
        options: { ...options, workspaceId },
      });
    },

    async getBuild(workspaceId: string, buildId: string) {
      return upstreamFetch<BuildVM>(`/api/v1/builds/${buildId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    buildStreamUrl(buildId: string) {
      return `${getUpstreamBaseUrl()}/api/v1/builds/${buildId}/stream`;
    },

    async abortBuild(workspaceId: string, buildId: string) {
      return upstreamFetch<{ aborted: true }>(`/api/v1/builds/${buildId}/abort`, {
        method: "POST",
        options: { ...options, workspaceId },
      });
    },

    async rollbackBuild(workspaceId: string, buildId: string) {
      return upstreamFetch<{
        id: string;
        status: string;
        build_number: number;
        rollback_of_build_id: string;
      }>(`/api/v1/builds/${buildId}/rollback`, {
        method: "POST",
        options: { ...options, workspaceId },
      });
    },

    async listCronJobs(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: CronJobVM[] }>(
        `/api/v1/projects/${projectId}/cronjobs`,
        {
          method: "GET",
          options: { ...options, workspaceId },
        },
      );
    },

    async createCronJob(
      workspaceId: string,
      projectId: string,
      input: { name: string; schedule: string; branch?: string },
    ) {
      return upstreamFetch<CronJobVM>(`/api/v1/projects/${projectId}/cronjobs`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async deleteCronJob(workspaceId: string, projectId: string, cronJobId: string) {
      return upstreamFetch<{ deleted: true }>(
        `/api/v1/projects/${projectId}/cronjobs/${cronJobId}`,
        {
          method: "DELETE",
          options: { ...options, workspaceId },
        },
      );
    },
  };
}

export type DeliveryUpstream = ReturnType<typeof createDeliveryUpstream>;
