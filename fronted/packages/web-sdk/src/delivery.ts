import type { BuildVM, CronJobVM } from "@sparkops/contracts/delivery";
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
