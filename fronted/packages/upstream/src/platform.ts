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

    async getWorkspaceServerStatus(wsId: string) {
      return upstreamFetch<ServerStatusVM>(
        `/api/v1/workspaces/${wsId}/monitor/server/status`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
    },

    async listOperationRecords(
      wsId: string,
      query: import("@sparkops/contracts/platform").ListOperationRecordsQuery,
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
      return upstreamFetch<import("@sparkops/contracts/platform").OperationRecordListVM>(
        `/api/v1/workspaces/${wsId}/operation-records${qs ? `?${qs}` : ""}`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
    },

    async deleteOperationRecord(wsId: string, recordId: string) {
      return upstreamFetch<{ deleted: boolean }>(
        `/api/v1/workspaces/${wsId}/operation-records/${recordId}`,
        { method: "DELETE", options: { ...options, workspaceId: wsId } },
      );
    },

    async batchDeleteOperationRecords(
      wsId: string,
      input: import("@sparkops/contracts/platform").BatchDeleteOperationRecordsCommand,
    ) {
      return upstreamFetch<{ deleted: number }>(
        `/api/v1/workspaces/${wsId}/operation-records/batch-delete`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId: wsId } },
      );
    },

    async listAppLogs(
      wsId: string,
      query: import("@sparkops/contracts/platform").ListAppLogsQuery,
    ) {
      const params = new URLSearchParams();
      if (query.page) params.set("page", String(query.page));
      if (query.page_size) params.set("page_size", String(query.page_size));
      if (query.level) params.set("level", query.level);
      if (query.category) params.set("category", query.category);
      if (query.trace_id) params.set("trace_id", query.trace_id);
      if (query.keyword) params.set("keyword", query.keyword);
      const qs = params.toString();
      return upstreamFetch<import("@sparkops/contracts/platform").AppLogListVM>(
        `/api/v1/workspaces/${wsId}/observability/logs${qs ? `?${qs}` : ""}`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
    },

    async listLogFiles(wsId: string) {
      return upstreamFetch<import("@sparkops/contracts/platform").LogFileListVM>(
        `/api/v1/workspaces/${wsId}/observability/log-files`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
    },

    async tailLogFile(wsId: string, fileName: string, offset = 0, limit = 200) {
      const params = new URLSearchParams();
      params.set("offset", String(offset));
      params.set("limit", String(limit));
      return upstreamFetch<import("@sparkops/contracts/platform").LogTailVM>(
        `/api/v1/workspaces/${wsId}/observability/log-files/${encodeURIComponent(fileName)}/tail?${params}`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
    },

    async getTraceDetail(wsId: string, traceId: string) {
      return upstreamFetch<import("@sparkops/contracts/platform").TraceDetailVM>(
        `/api/v1/workspaces/${wsId}/observability/traces/${encodeURIComponent(traceId)}`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
    },

    async getMetricsSummary(wsId: string) {
      return upstreamFetch<import("@sparkops/contracts/platform").MetricsSummaryVM>(
        `/api/v1/workspaces/${wsId}/observability/metrics/summary`,
        { method: "GET", options: { ...options, workspaceId: wsId } },
      );
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
