import type { BffContext } from "@sparkops/bff-core";
import type { CreateWorkflowCommand, UpdateWorkflowCommand, WorkflowRunVM, WorkflowVM } from "@sparkops/contracts/workflow";

function mapWorkflow(data: WorkflowVM): WorkflowVM {
  return {
    id: data.id,
    project_id: data.project_id,
    name: data.name,
    description: data.description ?? "",
    status: data.status,
    definition: data.definition,
  };
}

export async function listWorkflows(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<WorkflowVM[]> {
  const data = await ctx.upstream.workflow.listWorkflows(workspaceId, projectId);
  return data.items.map(mapWorkflow);
}

export async function createWorkflow(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: CreateWorkflowCommand,
): Promise<WorkflowVM> {
  const data = await ctx.upstream.workflow.createWorkflow(workspaceId, projectId, input);
  return mapWorkflow(data);
}

export async function getWorkflow(
  ctx: BffContext,
  workspaceId: string,
  workflowId: string,
): Promise<WorkflowVM> {
  const data = await ctx.upstream.workflow.getWorkflow(workspaceId, workflowId);
  return mapWorkflow(data);
}

export async function updateWorkflow(
  ctx: BffContext,
  workspaceId: string,
  workflowId: string,
  input: UpdateWorkflowCommand,
): Promise<WorkflowVM> {
  const data = await ctx.upstream.workflow.updateWorkflow(workspaceId, workflowId, input);
  return mapWorkflow(data);
}

export async function archiveWorkflow(
  ctx: BffContext,
  workspaceId: string,
  workflowId: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.workflow.archiveWorkflow(workspaceId, workflowId);
}

export async function activateWorkflow(
  ctx: BffContext,
  workspaceId: string,
  workflowId: string,
): Promise<WorkflowVM> {
  const data = await ctx.upstream.workflow.activateWorkflow(workspaceId, workflowId);
  return mapWorkflow(data);
}

export async function listWorkflowRuns(
  ctx: BffContext,
  workspaceId: string,
  workflowId: string,
): Promise<WorkflowRunVM[]> {
  const data = await ctx.upstream.workflow.listWorkflowRuns(workspaceId, workflowId);
  return data.items;
}

export async function startWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  workflowId: string,
  input?: string,
): Promise<WorkflowRunVM> {
  return ctx.upstream.workflow.startWorkflowRun(workspaceId, workflowId, input);
}

export async function getWorkflowRun(
  ctx: BffContext,
  workspaceId: string,
  runId: string,
): Promise<WorkflowRunVM> {
  return ctx.upstream.workflow.getWorkflowRun(workspaceId, runId);
}
