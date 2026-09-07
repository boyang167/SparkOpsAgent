import type { PodVM, ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";
import type { UpstreamRequestOptions } from "./core";
import { getUpstreamBaseUrl, upstreamFetch } from "./core";

export function createClusterUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listPods(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: PodVM[] }>(`/api/v1/projects/${projectId}/pods`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async getProjectRuntimeOverview(workspaceId: string, projectId: string) {
      return upstreamFetch<ProjectRuntimeOverviewVM>(
        `/api/v1/projects/${projectId}/runtime/overview`,
        {
          method: "GET",
          options: { ...options, workspaceId },
        },
      );
    },

    async getPod(workspaceId: string, projectId: string, podId: string) {
      return upstreamFetch<PodVM>(`/api/v1/projects/${projectId}/pods/${podId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async getPodLogs(workspaceId: string, projectId: string, podId: string) {
      return upstreamFetch<{ logs: string }>(
        `/api/v1/projects/${projectId}/pods/${podId}/logs`,
        {
          method: "GET",
          options: { ...options, workspaceId },
        },
      );
    },

    async restartPod(workspaceId: string, projectId: string, podId: string) {
      return upstreamFetch<{ restarted: boolean }>(
        `/api/v1/projects/${projectId}/pods/${podId}/restart`,
        {
          method: "POST",
          options: { ...options, workspaceId },
        },
      );
    },

    podTerminalStreamUrl(projectId: string, podId: string) {
      return `${getUpstreamBaseUrl()}/api/v1/projects/${projectId}/pods/${podId}/terminal/stream`;
    },
  };
}

export type ClusterUpstream = ReturnType<typeof createClusterUpstream>;
