import type { BffContext } from "@sparkops/bff-core";
import type { CreateProjectCommand, ProjectVM } from "@sparkops/contracts/workspace";

export async function createProject(
  ctx: BffContext,
  workspaceId: string,
  input: CreateProjectCommand,
): Promise<ProjectVM> {
  return ctx.upstream.workspace.createProject(workspaceId, input);
}
