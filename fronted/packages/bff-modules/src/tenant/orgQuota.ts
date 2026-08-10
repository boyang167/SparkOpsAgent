import type { BffContext } from "@sparkops/bff-core";
import type { OrgQuotaUsageVM, OrgQuotaVM, UpdateOrgQuotaCommand } from "@sparkops/contracts/tenant";

export async function getOrgQuota(ctx: BffContext, orgId: string): Promise<OrgQuotaVM> {
  return ctx.upstream.tenant.getOrgQuota(orgId);
}

export async function getOrgQuotaUsage(ctx: BffContext, orgId: string): Promise<OrgQuotaUsageVM> {
  return ctx.upstream.tenant.getOrgQuotaUsage(orgId);
}

export async function updateOrgQuota(
  ctx: BffContext,
  orgId: string,
  input: UpdateOrgQuotaCommand,
): Promise<OrgQuotaVM> {
  return ctx.upstream.tenant.updateOrgQuota(orgId, input);
}
