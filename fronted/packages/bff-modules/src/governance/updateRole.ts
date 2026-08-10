import type { BffContext } from "@sparkops/bff-core";
import type { RoleCreatedVM, UpdateRoleCommand } from "@sparkops/contracts/governance";

export async function updateRole(
  ctx: BffContext,
  workspaceId: string,
  roleId: string,
  input: UpdateRoleCommand,
): Promise<RoleCreatedVM> {
  return ctx.upstream.governance.updateRole(workspaceId, roleId, input);
}
