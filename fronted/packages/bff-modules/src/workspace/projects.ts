import type { BffContext } from "@sparkops/bff-core";
import type { ProjectVM } from "@sparkops/contracts/workspace";

export async function listProjects(ctx: BffContext, workspaceId: string): Promise<ProjectVM[]> {
  const data = await ctx.upstream.workspace.listProjects(workspaceId);
  return data.items;
}

export async function getProject(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<ProjectVM> {
  return ctx.upstream.workspace.getProject(workspaceId, projectId);
}
