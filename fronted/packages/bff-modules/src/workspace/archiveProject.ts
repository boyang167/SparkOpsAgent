import type { BffContext } from "@sparkops/bff-core";

export async function archiveProject(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<{ deleted: true }> {
  return ctx.upstream.workspace.archiveProject(workspaceId, projectId);
}
