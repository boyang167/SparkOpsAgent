import type { BffContext } from "@sparkops/bff-core";
import type { InviteMemberCommand, InvitedMemberVM } from "@sparkops/contracts/governance";

export async function inviteMember(
  ctx: BffContext,
  workspaceId: string,
  input: InviteMemberCommand,
): Promise<InvitedMemberVM> {
  return ctx.upstream.governance.inviteMember(workspaceId, input);
}
