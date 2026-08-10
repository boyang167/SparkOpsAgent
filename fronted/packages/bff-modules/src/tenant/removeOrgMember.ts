import type { BffContext } from "@sparkops/bff-core";

export async function removeOrgMember(ctx: BffContext, orgId: string, memberId: string) {
  return ctx.upstream.tenant.removeOrgMember(orgId, memberId);
}
