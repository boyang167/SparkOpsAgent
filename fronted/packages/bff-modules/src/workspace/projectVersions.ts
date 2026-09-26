import type { BffContext } from "@sparkops/bff-core";
import type {
  ProjectVersionDetailVM,
  ProjectVersionListVM,
} from "@sparkops/contracts/workspace";

export async function listProjectVersions(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<ProjectVersionListVM> {
  return ctx.upstream.workspace.listProjectVersions(workspaceId, projectId);
}

export async function getProjectVersion(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  versionId: string,
): Promise<ProjectVersionDetailVM> {
  return ctx.upstream.workspace.getProjectVersion(workspaceId, projectId, versionId);
}
