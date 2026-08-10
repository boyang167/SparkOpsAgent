import type { BffContext } from "@sparkops/bff-core";
import type { CreateOrganizationCommand, OrganizationVM } from "@sparkops/contracts/tenant";

export async function createOrganization(
  ctx: BffContext,
  input: CreateOrganizationCommand,
): Promise<OrganizationVM> {
  return ctx.upstream.tenant.createOrganization(input);
}
