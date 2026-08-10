import type { BffContext } from "@sparkops/bff-core";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";

export async function listWorkspaces(ctx: BffContext): Promise<WorkspaceVM[]> {
  const data = await ctx.upstream.tenant.listWorkspaces();
  return data.items;
}
