import type { BffContext } from "@sparkops/bff-core";
import type { OrganizationVM } from "@sparkops/contracts/tenant";

export async function getOrganization(ctx: BffContext, orgId: string): Promise<OrganizationVM & { my_role?: string }> {
  return ctx.upstream.tenant.getOrganization(orgId);
}
