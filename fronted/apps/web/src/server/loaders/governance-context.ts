import { redirect } from "next/navigation";
import type { MeVM } from "@sparkops/contracts/identity";
import type { WorkspaceMemberVM, RoleVM } from "@sparkops/contracts/governance";
import { loadSession } from "./session";
import { loadResolvedWorkspaceId } from "./workspace-context";
import { loadMyPermissions, loadRoles, loadMembers } from "./governance";

export interface GovernanceContext {
  session: MeVM;
  workspaceId: string;
  canManage: boolean;
  permissions: string[];
  roles: RoleVM[];
  members: WorkspaceMemberVM[];
}

export async function loadGovernanceContext(): Promise<GovernanceContext> {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { workspaceId } = await loadResolvedWorkspaceId();
  if (!workspaceId) {
    redirect("/dashboard");
  }

  let permissions: string[] = [];
  try {
    permissions = await loadMyPermissions(workspaceId);
  } catch {
    permissions = [];
  }

  const canManage = permissions.includes("governance:admin");
  let roles: RoleVM[] = [];
  let members: WorkspaceMemberVM[] = [];

  if (canManage) {
    try {
      [roles, members] = await Promise.all([loadRoles(workspaceId), loadMembers(workspaceId)]);
    } catch {
      roles = [];
      members = [];
    }
  }

  return { session, workspaceId, canManage, permissions, roles, members };
}
