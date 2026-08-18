import type { BffContext } from "@sparkops/bff-core";
import type { GeneratedTemplateListVM } from "@sparkops/contracts/workspace";

export async function listGeneratedTemplates(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<GeneratedTemplateListVM> {
  return ctx.upstream.workspace.listGeneratedTemplates(workspaceId, projectId);
}
