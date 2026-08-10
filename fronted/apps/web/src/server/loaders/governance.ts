import { getMyPermissions, listMembers, listRoles } from "@sparkops/bff-modules/governance";
import type { RoleVM, WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { loadBffContext } from "./context";

export async function loadRoles(workspaceId: string): Promise<RoleVM[]> {
  const ctx = await loadBffContext();
  return listRoles(ctx, workspaceId);
}

export async function loadMembers(workspaceId: string): Promise<WorkspaceMemberVM[]> {
  const ctx = await loadBffContext();
  return listMembers(ctx, workspaceId);
}

export async function loadMyPermissions(workspaceId: string): Promise<string[]> {
  const ctx = await loadBffContext();
  return getMyPermissions(ctx, workspaceId);
}
