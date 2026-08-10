import type { BffContext } from "@sparkops/bff-core";
import type { OrgMemberVM } from "@sparkops/contracts/tenant";

export async function listOrgMembers(ctx: BffContext, orgId: string): Promise<OrgMemberVM[]> {
  const data = await ctx.upstream.tenant.listOrgMembers(orgId);
  return data.items;
}
