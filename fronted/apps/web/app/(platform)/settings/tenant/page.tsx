import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { TenantManagementPanel } from "@/modules/tenant/components/TenantManagementPanel";
import { loadOrganizations } from "@/server/loaders/organizations";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";

export default async function TenantSettingsPage() {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const [organizations, workspaces] = await Promise.all([
    loadOrganizations(),
    loadWorkspaces(),
  ]);
  const cookieStore = await cookies();
  const currentWorkspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ?? workspaces[0]?.id;

  return (
    <TenantManagementPanel
      organizations={organizations}
      workspaces={workspaces}
      currentWorkspaceId={currentWorkspaceId}
    />
  );
}
