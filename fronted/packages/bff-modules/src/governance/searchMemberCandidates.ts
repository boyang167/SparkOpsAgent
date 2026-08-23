import type { BffContext } from "@sparkops/bff-core";
import type { MemberCandidateVM } from "@sparkops/contracts/governance";

export async function searchMemberCandidates(
  ctx: BffContext,
  workspaceId: string,
  q: string,
  limit = 10,
): Promise<MemberCandidateVM[]> {
  const data = await ctx.upstream.governance.searchMemberCandidates(workspaceId, q, limit);
  return data.items;
}
