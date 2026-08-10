import type { BffContext } from "@sparkops/bff-core";
import type { CreateRoleCommand, RoleCreatedVM } from "@sparkops/contracts/governance";

export async function createRole(
  ctx: BffContext,
  workspaceId: string,
  input: CreateRoleCommand,
): Promise<RoleCreatedVM> {
  return ctx.upstream.governance.createRole(workspaceId, input);
}
