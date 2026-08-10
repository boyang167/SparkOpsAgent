import type { BffContext } from "@sparkops/bff-core";

export async function deleteRole(
  ctx: BffContext,
  workspaceId: string,
  roleId: string,
): Promise<{ deleted: boolean }> {
  return ctx.upstream.governance.deleteRole(workspaceId, roleId);
}
