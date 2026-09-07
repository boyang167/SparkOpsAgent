import type { PodVM, ProjectRuntimeOverviewVM } from "@sparkops/contracts/cluster";
import { bffFetch } from "./client";

export async function listPods(projectId: string, workspaceId: string) {
  const data = await bffFetch<{ items: PodVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/pods`,
    { workspaceId },
  );
  return data.items;
}

export function getProjectRuntimeOverview(projectId: string, workspaceId: string) {
  return bffFetch<ProjectRuntimeOverviewVM>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/overview`,
    { workspaceId },
  );
}

export function getPod(projectId: string, podId: string, workspaceId: string) {
  return bffFetch<PodVM>(`/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}`, {
    workspaceId,
  });
}

export function getPodLogs(projectId: string, podId: string, workspaceId: string) {
  return bffFetch<{ logs: string }>(
    `/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}/logs`,
    { workspaceId },
  );
}

export function restartPod(projectId: string, podId: string, workspaceId: string) {
  return bffFetch<{ restarted: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}/restart`,
    { workspaceId, method: "POST" },
  );
}

export interface StreamPodTerminalHandlers {
  onDelta: (delta: string) => void;
  onDone: () => void;
  onError?: (error: Error) => void;
}

export function streamPodTerminal(
  projectId: string,
  podId: string,
  workspaceId: string,
  handlers: StreamPodTerminalHandlers,
): () => void {
  const es = new EventSource(
    `/api/bff/v1/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}/terminal/stream`,
  );

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
