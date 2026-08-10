import type { BffContext } from "@sparkops/bff-core";

export async function getMyOrgPermissions(ctx: BffContext, orgId: string): Promise<string[]> {
  const data = await ctx.upstream.tenant.getMyOrgPermissions(orgId);
  return data.permissions;
}
