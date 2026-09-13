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

export function getDeploymentYAML(projectId: string, workspaceId: string) {
  return bffFetch<{ yaml: string }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/deployment/yaml`,
    { workspaceId },
  );
}

export function getPod(projectId: string, podId: string, workspaceId: string) {
  return bffFetch<PodVM>(`/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}`, {
    workspaceId,
  });
}

export function getPodYAML(projectId: string, podId: string, workspaceId: string) {
  return bffFetch<{ yaml: string }>(
    `/workspaces/${workspaceId}/projects/${projectId}/pods/${podId}/yaml`,
    { workspaceId },
  );
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

export function putLogCollection(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").LogCollectionInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/log-collection`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function putHealthProbe(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").HealthProbeInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/health-probe`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function putServicePorts(
  projectId: string,
  workspaceId: string,
  body: { ports: import("@sparkops/contracts/cluster").RuntimeServicePortInput[] },
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/service/ports`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function createRuntimeService(projectId: string, workspaceId: string) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/service`,
    { workspaceId, method: "POST" },
  );
}

export function createRuntimeIngress(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").RuntimeIngressInput = {},
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/ingress`,
    { workspaceId, method: "POST", body: JSON.stringify(body) },
  );
}

export function scaleDeployment(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").ScaleDeploymentInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/deployment/scale`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function putDeploymentResources(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").DeploymentResourcesInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/deployment/resources`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export async function listConfigMaps(projectId: string, workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/cluster").ProjectConfigMapVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/configmaps`,
    { workspaceId },
  );
  return data.items ?? [];
}

export function createConfigMap(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").CreateProjectConfigMapInput = {},
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/configmaps`,
    { workspaceId, method: "POST", body: JSON.stringify(body) },
  );
}

export function upsertConfigMapEntry(
  projectId: string,
  workspaceId: string,
  name: string,
  body: import("@sparkops/contracts/cluster").UpsertConfigMapEntryInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/configmaps/${encodeURIComponent(name)}/data`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function replaceConfigMapData(
  projectId: string,
  workspaceId: string,
  name: string,
  body: import("@sparkops/contracts/cluster").ReplaceConfigMapDataInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/configmaps/${encodeURIComponent(name)}`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function deleteConfigMapEntry(
  projectId: string,
  workspaceId: string,
  name: string,
  key: string,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/configmaps/${encodeURIComponent(name)}/data?key=${encodeURIComponent(key)}`,
    { workspaceId, method: "DELETE" },
  );
}

export async function listPVCs(projectId: string, workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/cluster").ProjectPVCVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/pvcs`,
    { workspaceId },
  );
  return data.items ?? [];
}

export function createPVC(
  projectId: string,
  workspaceId: string,
  body: import("@sparkops/contracts/cluster").CreateProjectPVCInput = {},
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/pvcs`,
    { workspaceId, method: "POST", body: JSON.stringify(body) },
  );
}

export function deletePVC(projectId: string, workspaceId: string, name: string) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/pvcs/${encodeURIComponent(name)}`,
    { workspaceId, method: "DELETE" },
  );
}

export function bindPVC(
  projectId: string,
  workspaceId: string,
  name: string,
  body: import("@sparkops/contracts/cluster").BindProjectPVCInput,
) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/pvcs/${encodeURIComponent(name)}/bind`,
    { workspaceId, method: "PUT", body: JSON.stringify(body) },
  );
}

export function unbindPVC(projectId: string, workspaceId: string, name: string) {
  return bffFetch<{ ok: boolean }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/pvcs/${encodeURIComponent(name)}/bind`,
    { workspaceId, method: "DELETE" },
  );
}

export async function listStorageClasses(projectId: string, workspaceId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/cluster").StorageClassVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/runtime/storageclasses`,
    { workspaceId },
  );
  return data.items ?? [];
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
