import { redirect } from "next/navigation";
import type { ReactNode } from "react";
import type { WorkspaceMemberVM } from "@sparkops/contracts/governance";
import { InlineAlert } from "@/components/ui/InlineAlert";
import {
  hasFilesystemPageAccess,
  resolveWorkbenchPermissions,
} from "@/modules/filesystem/components/workbench-permissions";
import { loadSession } from "@/server/loaders/session";
import { loadMyPermissions, loadMembers } from "@/server/loaders/governance";
import { loadTenantContext } from "@/server/loaders/tenant-context";
import { loadMyOrganizationPermissions } from "@/server/loaders/organization-permissions";

export type FilesystemConsoleContext = {
  session: NonNullable<Awaited<ReturnType<typeof loadSession>>>;
  workspaceId: string;
  organizationId: string;
  organizationLabel: string;
  workspaceLabel: string;
  permissions: string[];
  members: WorkspaceMemberVM[];
  memberDirectoryAvailable: boolean;
};

export type FilesystemConsoleContextResult =
  | { ok: true; value: FilesystemConsoleContext; permissions: string[] }
  | { ok: false; element: ReactNode; permissions: string[] };

export async function loadFilesystemConsoleContext(): Promise<FilesystemConsoleContextResult> {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }
  const tenantContext = await loadTenantContext();
  const { workspaceId, currentWorkspace, currentOrganization } = tenantContext;
  if (!workspaceId) {
    redirect("/dashboard");
  }
  if (
    !currentWorkspace ||
    currentWorkspace.is_member === false ||
    !currentOrganization
  ) {
    return {
      ok: false,
      permissions: [],
      element: (
        <InlineAlert tone="error">
          你尚未加入当前工作区，或该工作区不属于可访问的租户。
        </InlineAlert>
      ),
    };
  }

  let workspacePermissions: string[] | undefined;
  try {
    workspacePermissions = await loadMyPermissions(workspaceId);
  } catch {
    workspacePermissions = undefined;
  }
  let organizationPermissions: string[] = [];
  try {
    organizationPermissions = await loadMyOrganizationPermissions(currentOrganization.id);
  } catch {
    organizationPermissions = [];
  }
  const permissions = resolveWorkbenchPermissions(
    workspacePermissions,
    session.is_super_admin ?? false,
    organizationPermissions,
  );

  if (!hasFilesystemPageAccess(permissions)) {
    return {
      ok: false,
      permissions,
      element: (
        <InlineAlert tone="error">
          无权访问文件中心。请联系工作区管理员授予文件读取权限。
        </InlineAlert>
      ),
    };
  }

  let members: WorkspaceMemberVM[] = [];
  let memberDirectoryAvailable = false;
  try {
    members = await loadMembers(workspaceId);
    memberDirectoryAvailable = true;
  } catch {
    members = [];
  }
  if (!members.some((member) => member.member_id === session.id)) {
    members.push({
      member_id: session.id,
      email: session.email,
      display_name: session.display_name,
      roles: [],
    });
  }

  return {
    ok: true,
    permissions,
    value: {
      session,
      workspaceId,
      organizationId: currentOrganization.id,
      organizationLabel: currentOrganization.name,
      workspaceLabel: currentWorkspace.name,
      permissions,
      members,
      memberDirectoryAvailable,
    },
  };
}