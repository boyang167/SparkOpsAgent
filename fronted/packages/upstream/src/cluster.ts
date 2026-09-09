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

    async putLogCollection(workspaceId: string, projectId: string, body: Record<string, unknown>) {
      return upstreamFetch<{ ok: boolean }>(`/api/v1/projects/${projectId}/runtime/log-collection`, {
        method: "PUT",
        body: JSON.stringify(body),
        options: { ...options, workspaceId },
      });
    },

    async putHealthProbe(workspaceId: string, projectId: string, body: Record<string, unknown>) {
      return upstreamFetch<{ ok: boolean }>(`/api/v1/projects/${projectId}/runtime/health-probe`, {
        method: "PUT",
        body: JSON.stringify(body),
        options: { ...options, workspaceId },
      });
    },

    async putServicePorts(workspaceId: string, projectId: string, body: Record<string, unknown>) {
      return upstreamFetch<{ ok: boolean }>(`/api/v1/projects/${projectId}/runtime/service/ports`, {
        method: "PUT",
        body: JSON.stringify(body),
        options: { ...options, workspaceId },
      });
    },

    async createRuntimeService(workspaceId: string, projectId: string) {
      return upstreamFetch<{ ok: boolean }>(`/api/v1/projects/${projectId}/runtime/service`, {
        method: "POST",
        options: { ...options, workspaceId },
      });
    },

    async createRuntimeIngress(workspaceId: string, projectId: string, body: Record<string, unknown> = {}) {
      return upstreamFetch<{ ok: boolean }>(`/api/v1/projects/${projectId}/runtime/ingress`, {
        method: "POST",
        body: JSON.stringify(body),
        options: { ...options, workspaceId },
      });
    },

    podTerminalStreamUrl(projectId: string, podId: string) {
      return `${getUpstreamBaseUrl()}/api/v1/projects/${projectId}/pods/${podId}/terminal/stream`;
    },

    async createPodTerminalTicket(
      workspaceId: string,
      projectId: string,
      podId: string,
      body: Record<string, unknown>,
    ) {
      return upstreamFetch<{
        ticket: string;
        expires_at: string;
        websocket_path: string;
      }>(`/api/v1/projects/${projectId}/pods/${podId}/terminal-ticket`, {
        method: "POST",
        body: JSON.stringify(body),
        options: { ...options, workspaceId },
      });
    },
  };
}

export type ClusterUpstream = ReturnType<typeof createClusterUpstream>;
