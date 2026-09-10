import type {
  BuildVM,
  CreateDeploymentCommand,
  CronJobVM,
  DeploymentOptionsVM,
  DeploymentRunListVM,
  DeploymentRunVM,
  ProjectWebhookVM,
  RollbackDeploymentCommand,
  RollbackPreviewVM,
  UpsertProjectWebhookInput,
  WebhookEventCatalogItem,
} from "@sparkops/contracts/delivery";
import { DeploymentEventVMSchema } from "@sparkops/contracts/delivery";
import { bffFetch } from "./client";

export async function listBuilds(projectId: string, workspaceId: string) {
  const data = await bffFetch<{ items: BuildVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/builds`,
    { workspaceId },
  );
  return data.items;
}

export function triggerBuild(projectId: string, workspaceId: string, branch?: string) {
  return bffFetch<BuildVM>(`/workspaces/${workspaceId}/projects/${projectId}/builds`, {
    method: "POST",
    body: JSON.stringify({ branch: branch ?? "main" }),
    workspaceId,
  });
}

export function getBuild(buildId: string, workspaceId: string) {
  return bffFetch<BuildVM>(`/builds/${buildId}`, { workspaceId });
}

export function abortBuild(buildId: string, workspaceId: string) {
  return bffFetch<{ aborted: true }>(`/builds/${buildId}/abort`, {
    method: "POST",
    workspaceId,
  });
}

export type RollbackBuildResult = {
  id: string;
  status: string;
  build_number: number;
  rollback_of_build_id: string;
};

export function rollbackBuild(buildId: string, workspaceId: string) {
  return bffFetch<RollbackBuildResult>(`/builds/${buildId}/rollback`, {
    method: "POST",
    workspaceId,
  });
}

export async function listDeployments(projectId: string, workspaceId: string) {
  return bffFetch<DeploymentRunListVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/deployments`,
    { workspaceId },
  );
}

export function createDeployment(
  projectId: string,
  workspaceId: string,
  input: CreateDeploymentCommand,
) {
  return bffFetch<DeploymentRunVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/deployments`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function rollbackDeployment(
  projectId: string,
  workspaceId: string,
  input: RollbackDeploymentCommand = {},
) {
  return bffFetch<DeploymentRunVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/deployments/rollback`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}

export function getRollbackPreview(
  projectId: string,
  workspaceId: string,
  targetRunId?: string,
) {
  const query = targetRunId ? `?target_run_id=${encodeURIComponent(targetRunId)}` : "";
  return bffFetch<RollbackPreviewVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/deployments/rollback-preview${query}`,
    { workspaceId },
  );
}

export function getDeployment(runId: string, workspaceId: string) {
  return bffFetch<DeploymentRunVM>(`/deployments/${runId}`, { workspaceId });
}

export function abortDeployment(runId: string, workspaceId: string) {
  return bffFetch<DeploymentRunVM>(`/deployments/${runId}/abort`, {
    method: "POST",
    workspaceId,
  });
}

export function getDeploymentOptions(projectId: string, workspaceId: string) {
  return bffFetch<DeploymentOptionsVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/deployments/options`,
    { workspaceId },
  );
}

export interface DeploymentEvent {
  delta?: string;
  done?: true;
}

export function parseDeploymentEvent(data: string): DeploymentEvent {
  const payload = DeploymentEventVMSchema.parse(JSON.parse(data));
  if (payload.done) return { done: true };
  return typeof payload.delta === "string" ? { delta: payload.delta } : {};
}

export interface StreamDeploymentHandlers {
  onDelta: (delta: string) => void;
  onDone: () => void;
  onError?: (error: Error) => void;
}

export function streamDeployment(
  runId: string,
  handlers: StreamDeploymentHandlers,
): () => void {
  const es = new EventSource(`/api/bff/v1/deployments/${runId}/stream`);
  es.onmessage = (event) => {
    try {
      const payload = parseDeploymentEvent(event.data);
      if (payload.done) {
        handlers.onDone();
        es.close();
      } else if (payload.delta) {
        handlers.onDelta(payload.delta);
      }
    } catch (err) {
      handlers.onError?.(err instanceof Error ? err : new Error("stream parse error"));
    }
  };
  es.onerror = () => {
    handlers.onError?.(new Error("stream connection error"));
    es.close();
  };
  return () => es.close();
}

export interface StreamBuildHandlers {
  onDelta: (delta: string) => void;
  onDone: () => void;
  onError?: (error: Error) => void;
}

export function streamBuild(buildId: string, handlers: StreamBuildHandlers): () => void {
  const es = new EventSource(`/api/bff/v1/builds/${buildId}/stream`);

  es.onmessage = (event) => {
    try {
      const payload = JSON.parse(event.data) as { delta?: string; done?: string };
      if (payload.done) {
        handlers.onDone();
        es.close();
        return;
      }
      if (payload.delta) {
        handlers.onDelta(payload.delta);
      }
    } catch (err) {
      handlers.onError?.(err instanceof Error ? err : new Error("stream parse error"));
    }
  };

  es.onerror = () => {
    handlers.onError?.(new Error("stream connection error"));
    es.close();
  };

  return () => es.close();
}

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

export async function listWebhooks(workspaceId: string, projectId: string) {
  return bffFetch<{ items: ProjectWebhookVM[]; catalog: WebhookEventCatalogItem[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/webhooks`,
    { workspaceId },
  );
}

export function createWebhook(workspaceId: string, projectId: string, input: UpsertProjectWebhookInput) {
  return bffFetch<ProjectWebhookVM>(`/workspaces/${workspaceId}/projects/${projectId}/webhooks`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function updateWebhook(
  workspaceId: string,
  projectId: string,
  webhookId: string,
  input: UpsertProjectWebhookInput,
) {
  return bffFetch<ProjectWebhookVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/webhooks/${webhookId}`,
    {
      method: "PUT",
      body: JSON.stringify(input),
      workspaceId,
    },
  );
}

export function deleteWebhook(workspaceId: string, projectId: string, webhookId: string) {
  return bffFetch<{ deleted: true }>(
    `/workspaces/${workspaceId}/projects/${projectId}/webhooks/${webhookId}`,
    {
      method: "DELETE",
      workspaceId,
    },
  );
}
