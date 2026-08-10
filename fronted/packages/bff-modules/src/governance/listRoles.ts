import type { BffContext } from "@sparkops/bff-core";
import type { RoleVM } from "@sparkops/contracts/governance";

export async function listRoles(ctx: BffContext, workspaceId: string): Promise<RoleVM[]> {
  const data = await ctx.upstream.governance.listRoles(workspaceId);
  return data.items;
}
