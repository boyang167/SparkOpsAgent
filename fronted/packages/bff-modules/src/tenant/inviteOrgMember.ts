import type { BffContext } from "@sparkops/bff-core";
import type { InviteOrgMemberCommand } from "@sparkops/contracts/tenant";

export async function inviteOrgMember(ctx: BffContext, orgId: string, input: InviteOrgMemberCommand) {
  return ctx.upstream.tenant.inviteOrgMember(orgId, input);
}
