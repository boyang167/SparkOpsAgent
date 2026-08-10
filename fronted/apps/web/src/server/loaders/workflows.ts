import { listWorkflows } from "@sparkops/bff-modules/workflow";
import type { WorkflowVM } from "@sparkops/contracts/workflow";
import { loadBffContext } from "./context";

export async function loadWorkflows(
  workspaceId: string,
  projectId: string,
): Promise<WorkflowVM[]> {
  const ctx = await loadBffContext();
  return listWorkflows(ctx, workspaceId, projectId);
}
