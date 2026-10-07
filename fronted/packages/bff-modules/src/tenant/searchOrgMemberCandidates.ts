import type { BffContext } from "@sparkops/bff-core";
import type { MemberCandidateVM } from "@sparkops/contracts/governance";

export async function searchOrgMemberCandidates(
  ctx: BffContext,
  orgId: string,
  q: string,
  limit = 10,
): Promise<MemberCandidateVM[]> {
  const data = await ctx.upstream.tenant.searchOrgMemberCandidates(orgId, q, limit);
  return data.items;
}
