import type { BffContext } from "@sparkops/bff-core";
import type { UpdateOrgMemberRoleCommand } from "@sparkops/contracts/tenant";

export async function updateOrgMemberRole(
  ctx: BffContext,
  orgId: string,
  memberId: string,
  input: UpdateOrgMemberRoleCommand,
) {
  return ctx.upstream.tenant.updateOrgMemberRole(orgId, memberId, input);
}
