import type { BffContext } from "@sparkops/bff-core";
import type { DataResourceVM, RoleDataPolicyVM } from "@sparkops/contracts/governance";

export async function listDataResources(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.governance.listDataResources(workspaceId);
  return data.items as DataResourceVM[];
}

export async function listRoleDataPolicies(ctx: BffContext, workspaceId: string, roleId: string) {
  const data = await ctx.upstream.governance.listRoleDataPolicies(workspaceId, roleId);
  return data.items as RoleDataPolicyVM[];
}

export async function updateRoleDataPolicies(
  ctx: BffContext,
  workspaceId: string,
  roleId: string,
  items: RoleDataPolicyVM[],
) {
  return ctx.upstream.governance.updateRoleDataPolicies(workspaceId, roleId, { items });
}
