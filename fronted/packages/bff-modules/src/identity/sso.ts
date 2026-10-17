import type { BffContext } from "@sparkops/bff-core";
import type {
  AuthSessionVM,
  AuthProvidersVM,
  LDAPLoginCommand,
  SSOAdminConfigVM,
  TestSSOProviderCommand,
  TestSSOProviderResultVM,
  UpdateSSOConfigCommand,
} from "@sparkops/contracts/identity";

async function sessionFromToken(ctx: BffContext, token: string): Promise<AuthSessionVM> {
  const me = await ctx.upstream.identity.me(token);
  return {
    token,
    memberId: me.id,
    email: me.email,
    displayName: me.display_name,
  };
}

export async function getAuthProviders(ctx: BffContext): Promise<AuthProvidersVM> {
  return ctx.upstream.identity.getAuthProviders();
}

export async function startOIDCLogin(ctx: BffContext) {
  return ctx.upstream.identity.startOIDCLogin();
}

export async function completeOIDCLogin(
  ctx: BffContext,
  code: string,
  state: string,
): Promise<AuthSessionVM> {
  const data = await ctx.upstream.identity.completeOIDCCallback(code, state);
  return sessionFromToken(ctx, data.token);
}

export async function executeLDAPLogin(ctx: BffContext, input: LDAPLoginCommand): Promise<AuthSessionVM> {
  const data = await ctx.upstream.identity.ldapLogin(input);
  return sessionFromToken(ctx, data.token);
}

export async function getSSOAdminConfig(ctx: BffContext): Promise<SSOAdminConfigVM> {
  return ctx.upstream.identity.getSSOAdminConfig();
}

export async function testSSOProvider(
  ctx: BffContext,
  input: TestSSOProviderCommand,
): Promise<TestSSOProviderResultVM> {
  return ctx.upstream.identity.testSSOProvider(input);
}

export async function updateSSOAdminConfig(
  ctx: BffContext,
  input: UpdateSSOConfigCommand,
): Promise<SSOAdminConfigVM> {
  return ctx.upstream.identity.updateSSOAdminConfig(input);
}
