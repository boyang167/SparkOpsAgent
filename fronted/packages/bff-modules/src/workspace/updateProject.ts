import type { BffContext } from "@sparkops/bff-core";
import type { ProjectVM, UpdateProjectCommand } from "@sparkops/contracts/workspace";

export async function updateProject(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: UpdateProjectCommand,
): Promise<ProjectVM> {
  return ctx.upstream.workspace.updateProject(workspaceId, projectId, input);
}
