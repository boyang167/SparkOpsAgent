import type {
  ClusterVM,
  DashboardVM,
  DeploymentStatVM,
  MonitorOverviewVM,
  ProclaimVM,
  ServerStatusVM,
  SyncJobVM,
  SysConfigVM,
  ToolVM,
} from "@sparkops/contracts/platform";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createPlatformUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async getDashboard(workspaceId: string) {
      return upstreamFetch<DashboardVM>(`/api/v1/workspaces/${workspaceId}/dashboard`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async getMonitorOverview(workspaceId: string) {
      return upstreamFetch<MonitorOverviewVM>(
        `/api/v1/workspaces/${workspaceId}/monitor/overview`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listDeploymentStats(workspaceId: string) {
      return upstreamFetch<{ items: DeploymentStatVM[] }>(
        `/api/v1/workspaces/${workspaceId}/statistics/deployments`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listClusters() {
      return upstreamFetch<{ items: ClusterVM[] }>("/api/v1/admin/clusters", {
        method: "GET",
        options,
      });
    },

    async createCluster(input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; cluster_name: string }>("/api/v1/admin/clusters", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },

    async listSyncJobs() {
      return upstreamFetch<{ items: SyncJobVM[] }>("/api/v1/admin/sync/jobs", {
        method: "GET",
        options,
      });
    },

    async createSyncJob(input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; name: string; status: string }>(
        "/api/v1/admin/sync/jobs",
        {
          method: "POST",
          body: JSON.stringify(input),
          options,
        },
      );
    },

    async getSysConfig() {
      return upstreamFetch<SysConfigVM>("/api/v1/admin/sysconfig", {
        method: "GET",
        options,
      });
    },

    async updateSysConfig(input: Record<string, unknown>) {
      return upstreamFetch<SysConfigVM>("/api/v1/admin/sysconfig", {
        method: "PATCH",
        body: JSON.stringify(input),
        options,
      });
    },

    async getServerStatus() {
      return upstreamFetch<ServerStatusVM>("/api/v1/admin/server/status", {
        method: "GET",
        options,
      });
    },

    async listProclaims() {
      return upstreamFetch<{ items: ProclaimVM[] }>("/api/v1/admin/proclaims", {
        method: "GET",
        options,
      });
    },

    async createProclaim(input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; title: string }>("/api/v1/admin/proclaims", {
        method: "POST",
        body: JSON.stringify(input),
        options,
      });
    },

    async listTools() {
      return upstreamFetch<{ items: ToolVM[] }>("/api/v1/admin/tools", {
        method: "GET",
        options,
      });
    },
  };
}

export type PlatformUpstream = ReturnType<typeof createPlatformUpstream>;
