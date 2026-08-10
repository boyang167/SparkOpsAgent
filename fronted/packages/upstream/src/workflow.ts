import type { WorkflowRunVM, WorkflowVM } from "@sparkops/contracts/workflow";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createWorkflowUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listWorkflows(workspaceId: string, projectId: string) {
      return upstreamFetch<{ items: WorkflowVM[] }>(`/api/v1/projects/${projectId}/workflows`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async createWorkflow(
      workspaceId: string,
      projectId: string,
      input: { name: string; description?: string; definition?: string },
    ) {
      return upstreamFetch<WorkflowVM>(`/api/v1/projects/${projectId}/workflows`, {
        method: "POST",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async getWorkflow(workspaceId: string, workflowId: string) {
      return upstreamFetch<WorkflowVM>(`/api/v1/workflows/${workflowId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async updateWorkflow(
      workspaceId: string,
      workflowId: string,
      input: { name: string; description?: string; definition?: string },
    ) {
      return upstreamFetch<WorkflowVM>(`/api/v1/workflows/${workflowId}`, {
        method: "PATCH",
        body: JSON.stringify(input),
        options: { ...options, workspaceId },
      });
    },

    async archiveWorkflow(workspaceId: string, workflowId: string) {
      return upstreamFetch<{ deleted: true }>(`/api/v1/workflows/${workflowId}`, {
        method: "DELETE",
        options: { ...options, workspaceId },
      });
    },

    async activateWorkflow(workspaceId: string, workflowId: string) {
      return upstreamFetch<WorkflowVM>(`/api/v1/workflows/${workflowId}/activate`, {
        method: "POST",
        options: { ...options, workspaceId },
      });
    },

    async startWorkflowRun(workspaceId: string, workflowId: string, input?: string) {
      return upstreamFetch<WorkflowRunVM>(`/api/v1/workflows/${workflowId}/runs`, {
        method: "POST",
        body: JSON.stringify({ input: input ?? "" }),
        options: { ...options, workspaceId },
      });
    },

    async listWorkflowRuns(workspaceId: string, workflowId: string) {
      return upstreamFetch<{ items: WorkflowRunVM[] }>(`/api/v1/workflows/${workflowId}/runs`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },

    async getWorkflowRun(workspaceId: string, runId: string) {
      return upstreamFetch<WorkflowRunVM>(`/api/v1/workflow-runs/${runId}`, {
        method: "GET",
        options: { ...options, workspaceId },
      });
    },
  };
}

export type WorkflowUpstream = ReturnType<typeof createWorkflowUpstream>;
