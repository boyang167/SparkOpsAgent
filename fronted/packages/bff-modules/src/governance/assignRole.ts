import type { BffContext } from "@sparkops/bff-core";
import type { AssignRoleCommand } from "@sparkops/contracts/governance";

export async function assignRole(
  ctx: BffContext,
  workspaceId: string,
  memberId: string,
  input: AssignRoleCommand,
): Promise<{ assigned: boolean }> {
  return ctx.upstream.governance.assignRole(workspaceId, memberId, input);
}
