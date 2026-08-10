import { listOrganizations } from "@sparkops/bff-modules/tenant";
import type { OrganizationVM } from "@sparkops/contracts/tenant";
import { loadBffContext } from "./context";

export async function loadOrganizations(): Promise<OrganizationVM[]> {
  const ctx = await loadBffContext();
  return listOrganizations(ctx);
}
