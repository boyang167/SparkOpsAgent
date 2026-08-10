import type { BffContext } from "@sparkops/bff-core";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";

export async function listMembers(
  ctx: BffContext,
  workspaceId: string,
): Promise<WorkspaceMemberVM[]> {
  const data = await ctx.upstream.governance.listMembers(workspaceId);
  return data.items;
}
