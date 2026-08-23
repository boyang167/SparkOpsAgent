import { redirect } from "next/navigation";
import type { MeVM } from "@sparkops/contracts/identity";
import { loadMyPermissions } from "./governance";
import { loadSession } from "./session";
import { loadResolvedWorkspaceId } from "./workspace-context";

export interface ConsoleAccess {
  session: MeVM | null;
  isSuperAdmin: boolean;
  canManageAdmin: boolean;
}

export async function loadConsoleAccess(): Promise<ConsoleAccess> {
  const session = await loadSession();
  if (!session) {
    return { session: null, isSuperAdmin: false, canManageAdmin: false };
  }

  const isSuperAdmin = session.is_super_admin ?? false;

  let canManageAdmin = isSuperAdmin;
  if (!canManageAdmin) {
    try {
      const { workspaceId } = await loadResolvedWorkspaceId();
      if (workspaceId) {
        const permissions = await loadMyPermissions(workspaceId);
        canManageAdmin = permissions.includes("governance:admin");
      }
    } catch {
      canManageAdmin = false;
    }
  }

  return {
    session,
    isSuperAdmin,
    canManageAdmin,
  };
}

export async function requireAdminConsoleAccess(): Promise<ConsoleAccess & { session: MeVM }> {
  const access = await loadConsoleAccess();
  if (!access.session) {
    redirect("/login");
  }
  if (!access.canManageAdmin) {
    redirect("/dashboard/workplace");
  }
  return { ...access, session: access.session };
}

export async function requirePlatformConsoleAccess(): Promise<ConsoleAccess & { session: MeVM }> {
  const access = await loadConsoleAccess();
  if (!access.session) {
    redirect("/login");
  }
  if (!access.isSuperAdmin) {
    redirect("/dashboard/workplace");
  }
  return { ...access, session: access.session };
}
