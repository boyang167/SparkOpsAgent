import type { WorkflowRunVM, WorkflowVM } from "@sparkops/contracts/workflow";
import { bffFetch, type BffFetchOptions } from "./client";

export async function listWorkflows(workspaceId: string, projectId: string) {
  const options: BffFetchOptions = { workspaceId };
  const data = await bffFetch<{ items: WorkflowVM[] }>(
    `/workspaces/${workspaceId}/projects/${projectId}/workflows`,
    options,
  );
  return data.items;
}

export function createWorkflow(
  workspaceId: string,
  projectId: string,
  input: { name: string; description?: string; definition?: string },
) {
  return bffFetch<WorkflowVM>(`/workspaces/${workspaceId}/projects/${projectId}/workflows`, {
    method: "POST",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function getWorkflow(workflowId: string, workspaceId: string) {
  return bffFetch<WorkflowVM>(`/workflows/${workflowId}`, { workspaceId });
}

export function updateWorkflow(
  workflowId: string,
  workspaceId: string,
  input: { name: string; description?: string; definition?: string },
) {
  return bffFetch<WorkflowVM>(`/workflows/${workflowId}`, {
    method: "PATCH",
    body: JSON.stringify(input),
    workspaceId,
  });
}

export function archiveWorkflow(workflowId: string, workspaceId: string) {
  return bffFetch<{ deleted: true }>(`/workflows/${workflowId}`, {
    method: "DELETE",
    workspaceId,
  });
}

export function activateWorkflow(workflowId: string, workspaceId: string) {
  return bffFetch<WorkflowVM>(`/workflows/${workflowId}/activate`, {
    method: "POST",
    workspaceId,
  });
}

export async function listWorkflowRuns(workflowId: string, workspaceId: string) {
  const data = await bffFetch<{ items: WorkflowRunVM[] }>(`/workflows/${workflowId}/runs`, {
    workspaceId,
  });
  return data.items;
}

export function startWorkflowRun(workflowId: string, workspaceId: string, input?: string) {
  return bffFetch<WorkflowRunVM>(`/workflows/${workflowId}/runs`, {
    method: "POST",
    body: JSON.stringify({ input: input ?? "" }),
    workspaceId,
  });
}

export function getWorkflowRun(runId: string, workspaceId: string) {
  return bffFetch<WorkflowRunVM>(`/workflow-runs/${runId}`, { workspaceId });
}
