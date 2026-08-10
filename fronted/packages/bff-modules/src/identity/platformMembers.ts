import type { BffContext } from "@sparkops/bff-core";
import type {
  AdminResetPasswordCommand,
  CreatePlatformMemberCommand,
  PlatformMemberVM,
  UpdatePlatformMemberCommand,
} from "@sparkops/contracts/identity";

export async function listPlatformMembers(ctx: BffContext): Promise<PlatformMemberVM[]> {
  const data = await ctx.upstream.identity.listPlatformMembers();
  return data.items;
}

export async function createPlatformMember(ctx: BffContext, input: CreatePlatformMemberCommand) {
  return ctx.upstream.identity.createPlatformMember(input);
}

export async function updatePlatformMember(
  ctx: BffContext,
  memberId: string,
  input: UpdatePlatformMemberCommand,
) {
  return ctx.upstream.identity.updatePlatformMember(memberId, input);
}

export async function resetPlatformMemberPassword(
  ctx: BffContext,
  memberId: string,
  input: AdminResetPasswordCommand,
) {
  return ctx.upstream.identity.resetPlatformMemberPassword(memberId, input);
}
