import { cache } from "react";
import type { OrganizationVM, WorkspaceVM } from "@sparkops/contracts/tenant";
import { loadOrganizations } from "./organizations";
import { loadResolvedWorkspaceId } from "./workspace-context";

export interface TenantContext {
  organizations: OrganizationVM[];
  workspaces: WorkspaceVM[];
  workspaceId: string | undefined;
  currentWorkspace: WorkspaceVM | undefined;
  currentOrganization: OrganizationVM | undefined;
}

export const loadTenantContext = cache(async (): Promise<TenantContext> => {
  const [organizations, resolved] = await Promise.all([
    loadOrganizations().catch(() => [] as OrganizationVM[]),
    loadResolvedWorkspaceId(),
  ]);

  const { workspaces, workspaceId } = resolved;
  const currentWorkspace = workspaces.find((ws) => ws.id === workspaceId);
  const currentOrganization = currentWorkspace
    ? organizations.find((org) => org.id === currentWorkspace.org_id)
    : undefined;

  return {
    organizations,
    workspaces,
    workspaceId,
    currentWorkspace,
    currentOrganization,
  };
});
