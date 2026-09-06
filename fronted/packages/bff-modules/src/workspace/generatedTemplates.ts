import type { BffContext } from "@sparkops/bff-core";
import type {
  GeneratedTemplateArtifact,
  GeneratedTemplateListVM,
  UpdateGeneratedTemplateCommand,
} from "@sparkops/contracts/workspace";

export async function listGeneratedTemplates(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<GeneratedTemplateListVM> {
  return ctx.upstream.workspace.listGeneratedTemplates(workspaceId, projectId);
}

export async function updateGeneratedTemplate(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  kind: string,
  input: UpdateGeneratedTemplateCommand,
): Promise<GeneratedTemplateArtifact> {
  return ctx.upstream.workspace.updateGeneratedTemplate(workspaceId, projectId, kind, input);
}
