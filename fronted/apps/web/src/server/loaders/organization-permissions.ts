import { getMyOrgPermissions } from "@sparkops/bff-modules/tenant";
import { loadBffContext } from "./context";

export async function loadMyOrganizationPermissions(organizationId: string): Promise<string[]> {
  const ctx = await loadBffContext();
  return getMyOrgPermissions(ctx, organizationId);
}
