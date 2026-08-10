import type { BffContext } from "@sparkops/bff-core";
import type {
  AddProjectMemberCommand,
  ProjectMemberVM,
  UpdateProjectMemberRoleCommand,
} from "@sparkops/contracts/workspace";

export async function listProjectMembers(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
): Promise<ProjectMemberVM[]> {
  const data = await ctx.upstream.workspace.listProjectMembers(workspaceId, projectId);
  return data.items;
}

export async function addProjectMember(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  input: AddProjectMemberCommand,
) {
  return ctx.upstream.workspace.addProjectMember(workspaceId, projectId, input);
}

export async function updateProjectMemberRole(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  memberId: string,
  input: UpdateProjectMemberRoleCommand,
) {
  return ctx.upstream.workspace.updateProjectMemberRole(workspaceId, projectId, memberId, input);
}

export async function removeProjectMember(
  ctx: BffContext,
  workspaceId: string,
  projectId: string,
  memberId: string,
) {
  return ctx.upstream.workspace.removeProjectMember(workspaceId, projectId, memberId);
}
