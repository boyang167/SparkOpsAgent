import type {
  BuildVM,
  CreateDeploymentCommand,
  CronJobVM,
  DeploymentOptionsVM,
  DeploymentRunListVM,
  DeploymentRunVM,
  RollbackDeploymentCommand,
  RollbackPreviewVM,
} from "@sparkops/contracts/delivery";
import { RollbackPreviewVMSchema } from "@sparkops/contracts/delivery";
import { DeploymentRunVMSchema } from "@sparkops/contracts/delivery";
import type { UpstreamRequestOptions } from "./core";
import { getUpstreamBaseUrl, upstreamFetch } from "./core";

interface DeploymentOptionsWire {
  clusters: Array<{
    id: string;
    name: string;
    server?: string;
    available: boolean;
    reason?: string;
  }>;
}

export function normalizeDeploymentOptions(input: DeploymentOptionsWire): DeploymentOptionsVM {
  return {
    clusters: input.clusters
      .filter((cluster) => cluster.available)
      .map((cluster) => ({
        id: cluster.id,
        cluster_name: cluster.name,
        server: cluster.server,
      })),
    missing: [
      ...new Set(
        input.clusters
          .filter((cluster) => !cluster.available)
          .map((cluster) => cluster.reason)
          .filter((reason): reason is string => Boolean(reason)),
      ),
    ],
  };
}

export function normalizeDeploymentRun(input: unknown): DeploymentRunVM {
  const wire = input as Record<string, unknown>;
  return DeploymentRunVMSchema.parse({
    ...wire,
    run_type: wire.run_type ?? "normal",
    current_stage: wire.current_stage ?? "",
    error_message: wire.error_message ?? "",
  });
}

export function normalizeDeploymentList(input: {
  items?: unknown[];
  total?: number;
  page?: number;
  page_size?: number;
}): DeploymentRunListVM {
  const items = (input.items ?? []).map(normalizeDeploymentRun);
  return {
    items,
    total: input.total ?? items.length,
    page: input.page ?? 1,
    page_size: input.page_size ?? 20,
  };
}

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

    async listDeployments(workspaceId: string, projectId: string) {
      const data = await upstreamFetch<{
        items?: unknown[];
        total?: number;
        page?: number;
        page_size?: number;
      }>(
        `/api/v1/projects/${projectId}/deployments`,
        { method: "GET", options: { ...options, workspaceId } },
      );
      return normalizeDeploymentList(data);
    },

    async createDeployment(
      workspaceId: string,
      projectId: string,
      input: CreateDeploymentCommand,
    ) {
      const data = await upstreamFetch<unknown>(`/api/v1/projects/${projectId}/deployments`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
      return normalizeDeploymentRun(data);
    },

    async rollbackDeployment(
      workspaceId: string,
      projectId: string,
      input: RollbackDeploymentCommand = {},
    ) {
      const data = await upstreamFetch<unknown>(
        `/api/v1/projects/${projectId}/deployments/rollback`,
        {
          method: "POST",
          body: JSON.stringify(input),
          options: { ...options, workspaceId },
        },
      );
      return normalizeDeploymentRun(data);
    },

    async getRollbackPreview(
      workspaceId: string,
      projectId: string,
      targetRunId?: string,
    ) {
      const query = targetRunId ? `?target_run_id=${encodeURIComponent(targetRunId)}` : "";
      const data = await upstreamFetch<unknown>(
        `/api/v1/projects/${projectId}/deployments/rollback-preview${query}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
      const wire = data as Record<string, unknown>;
      return RollbackPreviewVMSchema.parse({
        ...wire,
        current_run: wire.current_run
          ? normalizeDeploymentRun(wire.current_run)
          : undefined,
        target_run: wire.target_run ? normalizeDeploymentRun(wire.target_run) : undefined,
      });
    },

    async getDeployment(workspaceId: string, runId: string) {
      const data = await upstreamFetch<unknown>(`/api/v1/deployments/${runId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
      return normalizeDeploymentRun(data);
    },

    async abortDeployment(workspaceId: string, runId: string) {
      const data = await upstreamFetch<unknown>(`/api/v1/deployments/${runId}/abort`, {
        method: "POST",
        options: { ...options, workspaceId },
      });
      return normalizeDeploymentRun(data);
    },

    async getDeploymentOptions(workspaceId: string, projectId: string) {
      const data = await upstreamFetch<DeploymentOptionsWire>(
        `/api/v1/projects/${projectId}/deployments/options`,
        { method: "GET", options: { ...options, workspaceId } },
      );
      return normalizeDeploymentOptions(data);
    },

    deploymentStreamUrl(runId: string) {
      return `${getUpstreamBaseUrl()}/api/v1/deployments/${runId}/stream`;
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

    async listWebhooks(workspaceId: string, projectId: string) {
      return upstreamFetch<{
        items: import("@sparkops/contracts/delivery").ProjectWebhookVM[];
        catalog: import("@sparkops/contracts/delivery").WebhookEventCatalogItem[];
      }>(`/api/v1/projects/${projectId}/webhooks`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async createWebhook(
      workspaceId: string,
      projectId: string,
      input: import("@sparkops/contracts/delivery").UpsertProjectWebhookInput,
    ) {
      return upstreamFetch<import("@sparkops/contracts/delivery").ProjectWebhookVM>(
        `/api/v1/projects/${projectId}/webhooks`,
        {
          method: "POST",
          body: JSON.stringify(input),
          options: { ...options, workspaceId },
        },
      );
    },

    async updateWebhook(
      workspaceId: string,
      projectId: string,
      webhookId: string,
      input: import("@sparkops/contracts/delivery").UpsertProjectWebhookInput,
    ) {
      return upstreamFetch<import("@sparkops/contracts/delivery").ProjectWebhookVM>(
        `/api/v1/projects/${projectId}/webhooks/${webhookId}`,
        {
          method: "PUT",
          body: JSON.stringify(input),
          options: { ...options, workspaceId },
        },
      );
    },

    async deleteWebhook(workspaceId: string, projectId: string, webhookId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/projects/${projectId}/webhooks/${webhookId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },
  };
}

export type DeliveryUpstream = ReturnType<typeof createDeliveryUpstream>;
