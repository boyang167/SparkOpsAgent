import { getWorkflow, getWorkflowRun, listWorkflowRuns } from "@sparkops/bff-modules/workflow";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { loadBffContext } from "./context";

export async function loadWorkflow(workspaceId: string, workflowId: string): Promise<WorkflowVM> {
  const ctx = await loadBffContext();
  return getWorkflow(ctx, workspaceId, workflowId);
}

export async function loadWorkflowRuns(workspaceId: string, workflowId: string) {
  const ctx = await loadBffContext();
  return listWorkflowRuns(ctx, workspaceId, workflowId);
}

export async function loadWorkflowRun(workspaceId: string, runId: string) {
  const ctx = await loadBffContext();
  return getWorkflowRun(ctx, workspaceId, runId);
}
