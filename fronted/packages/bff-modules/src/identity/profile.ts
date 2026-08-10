import type { BffContext } from "@sparkops/bff-core";
import type { ChangePasswordCommand, MeVM, UpdateProfileCommand } from "@sparkops/contracts/identity";

export async function updateProfile(ctx: BffContext, input: UpdateProfileCommand): Promise<MeVM> {
  if (!ctx.token) {
    throw new Error("authentication required");
  }
  const me = await ctx.upstream.identity.updateProfile(ctx.token, input);
  return {
    id: me.id,
    email: me.email,
    display_name: me.display_name,
    is_super_admin: me.is_super_admin ?? false,
  };
}

export async function changePassword(ctx: BffContext, input: ChangePasswordCommand): Promise<{ changed: true }> {
  if (!ctx.token) {
    throw new Error("authentication required");
  }
  return ctx.upstream.identity.changePassword(ctx.token, input);
}
