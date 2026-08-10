import type { BffContext } from "@sparkops/bff-core";
import type { OrganizationVM } from "@sparkops/contracts/tenant";

export async function listOrganizations(ctx: BffContext): Promise<OrganizationVM[]> {
  const data = await ctx.upstream.tenant.listOrganizations();
  return data.items;
}
