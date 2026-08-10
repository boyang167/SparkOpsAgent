import type { PodVM } from "@sparkops/contracts/cluster";
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

    podTerminalStreamUrl(projectId: string, podId: string) {
      return `${getUpstreamBaseUrl()}/api/v1/projects/${projectId}/pods/${podId}/terminal/stream`;
    },

    async listConfigMaps(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: Record<string, unknown>[] }>(
        `/api/v1/workspaces/${workspaceId}/k8s/projects/${projectId}/configmaps`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listIngresses(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: Record<string, unknown>[] }>(
        `/api/v1/workspaces/${workspaceId}/k8s/projects/${projectId}/ingresses`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listServices(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: Record<string, unknown>[] }>(
        `/api/v1/workspaces/${workspaceId}/k8s/projects/${projectId}/services`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listPVCs(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: Record<string, unknown>[] }>(
        `/api/v1/workspaces/${workspaceId}/k8s/projects/${projectId}/pvcs`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listVirtualServices(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: Record<string, unknown>[] }>(
        `/api/v1/workspaces/${workspaceId}/k8s/projects/${projectId}/virtualservices`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listConsulKV(workspaceId: string, prefix?: string) {
      const qs = prefix ? `?prefix=${encodeURIComponent(prefix)}` : "";
      return upstreamFetch<{ items: { key: string; value: string; updated_at?: string }[] }>(
        `/api/v1/workspaces/${workspaceId}/consul/kv${qs}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async putConsulKV(workspaceId: string, input: { key: string; value: string }) {
      return upstreamFetch<{ saved: boolean }>(
        `/api/v1/workspaces/${workspaceId}/consul/kv`,
        { method: "PUT", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteConsulKV(workspaceId: string, key: string) {
      return upstreamFetch<{ deleted: boolean }>(
        `/api/v1/workspaces/${workspaceId}/consul/kv?key=${encodeURIComponent(key)}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },
  };
}

export type ClusterUpstream = ReturnType<typeof createClusterUpstream>;
