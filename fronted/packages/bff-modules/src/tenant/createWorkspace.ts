import type { BffContext } from "@sparkops/bff-core";
import type { CreateWorkspaceCommand, WorkspaceCreatedVM } from "@sparkops/contracts/tenant";

export async function createWorkspace(
  ctx: BffContext,
  orgId: string,
  input: CreateWorkspaceCommand,
): Promise<WorkspaceCreatedVM> {
  return ctx.upstream.tenant.createWorkspace(orgId, input);
}
