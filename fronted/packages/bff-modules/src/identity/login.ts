import type { BffContext } from "@sparkops/bff-core";
import type { AuthSessionVM, LoginCommand, MeVM, RegisterCommand } from "@sparkops/contracts/identity";

export async function executeLogin(ctx: BffContext, input: LoginCommand): Promise<AuthSessionVM> {
  const data = await ctx.upstream.identity.login(input);
  const me = await ctx.upstream.identity.me(data.token);
  return {
    token: data.token,
    memberId: me.id,
    email: me.email,
    displayName: me.display_name,
  };
}

export async function executeRegister(ctx: BffContext, input: RegisterCommand): Promise<AuthSessionVM> {
  await ctx.upstream.identity.register(input);
  return executeLogin(ctx, { email: input.email, password: input.password });
}

export async function getSession(ctx: BffContext): Promise<MeVM | null> {
  if (!ctx.token) {
    return null;
  }
  try {
    const me = await ctx.upstream.identity.me(ctx.token);
    return {
      id: me.id,
      email: me.email,
      display_name: me.display_name,
      is_super_admin: me.is_super_admin ?? false,
    };
  } catch {
    return null;
  }
}
