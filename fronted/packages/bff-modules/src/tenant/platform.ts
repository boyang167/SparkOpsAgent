import type { BffContext } from "@sparkops/bff-core";
import type {
  CreatePlatformOrganizationCommand,
  PlatformOrganizationVM,
  TransferOrgOwnerCommand,
  UpdateOrgStatusCommand,
} from "@sparkops/contracts/tenant";

export async function listPlatformOrganizations(ctx: BffContext): Promise<PlatformOrganizationVM[]> {
  const data = await ctx.upstream.tenant.listPlatformOrganizations();
  return data.items;
}

export async function createPlatformOrganization(
  ctx: BffContext,
  input: CreatePlatformOrganizationCommand,
): Promise<PlatformOrganizationVM> {
  return ctx.upstream.tenant.createPlatformOrganization(input);
}

export async function updatePlatformOrganizationStatus(
  ctx: BffContext,
  orgId: string,
  input: UpdateOrgStatusCommand,
) {
  return ctx.upstream.tenant.updatePlatformOrganizationStatus(orgId, input);
}

export async function deletePlatformOrganization(ctx: BffContext, orgId: string) {
  return ctx.upstream.tenant.deletePlatformOrganization(orgId);
}

export async function transferPlatformOrgOwner(
  ctx: BffContext,
  orgId: string,
  input: TransferOrgOwnerCommand,
) {
  return ctx.upstream.tenant.transferPlatformOrgOwner(orgId, input);
}

export async function transferOrgOwner(ctx: BffContext, orgId: string, input: TransferOrgOwnerCommand) {
  return ctx.upstream.tenant.transferOrgOwner(orgId, input);
}

export async function listOrgAuditLogs(ctx: BffContext, orgId: string, limit?: number, offset?: number) {
  const data = await ctx.upstream.tenant.listOrgAuditLogs(orgId, { limit, offset });
  return data.items;
}

export async function listPlatformAuditLogs(ctx: BffContext, limit?: number, offset?: number) {
  const data = await ctx.upstream.tenant.listPlatformAuditLogs({ limit, offset });
  return data.items;
}
