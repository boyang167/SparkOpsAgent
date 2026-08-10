import type { BffContext } from "@sparkops/bff-core";

export async function removeRoleBinding(
  ctx: BffContext,
  workspaceId: string,
  memberId: string,
  roleId: string,
): Promise<{ removed: boolean }> {
  return ctx.upstream.governance.removeRoleBinding(workspaceId, memberId, roleId);
}
