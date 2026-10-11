import type { BffContext } from "@sparkops/bff-core";
import type { ArchiveProjectCommand, ArchiveProjectResultVM } from "@sparkops/contracts/workspace";

export async function archiveProjectWithTeardown(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: ArchiveProjectCommand,
): Promise<ArchiveProjectResultVM> {
  return ctx.upstream.workspace.archiveProjectWithTeardown(workspaceId, projectId, input);
}
