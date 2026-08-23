import { headers } from "next/headers";
import { AppShell } from "@/components/layout/AppShell";
import { extractProjectId } from "@/components/layout/context-utils";
import { SidebarNav } from "@/components/layout/SidebarNav";
import { resolveConsoleKind } from "@/components/layout/nav-tree";
import { loadConsoleAccess } from "@/server/loaders/console-access";
import { requirePathEntitlement } from "@/server/loaders/require-path-entitlement";
import { loadProjects } from "@/server/loaders/projects";
import { loadTenantContext } from "@/server/loaders/tenant-context";

export default async function PlatformLayout({ children }: { children: React.ReactNode }) {
  const headerStore = await headers();
  const pathname = headerStore.get("x-pathname") ?? "";
  const consoleKind = resolveConsoleKind(pathname);
  const projectId = extractProjectId(pathname);

  const { session, isSuperAdmin, canManageAdmin } = await loadConsoleAccess();

  let tenantContext: Awaited<ReturnType<typeof loadTenantContext>> | null = null;
  let projectOptions: { id: string; name: string }[] = [];

  if (session) {
    try {
      tenantContext = await loadTenantContext();
    } catch {
      tenantContext = null;
    }

    if (
      consoleKind === "business" &&
      tenantContext?.workspaceId &&
      pathname &&
      !pathname.startsWith("/dashboard/module-unavailable")
    ) {
      await requirePathEntitlement(pathname, tenantContext.workspaceId);
    }

    if (tenantContext?.workspaceId) {
      try {
        const projects = await loadProjects(tenantContext.workspaceId).catch(() => []);
        projectOptions = projects.map((item) => ({ id: item.id, name: item.name }));
      } catch {
        // ignore project load errors
      }
    }
  }

  return (
    <AppShell
      nav={
        <SidebarNav workspaceId={tenantContext?.workspaceId} projects={projectOptions} />
      }
      session={session}
      organizations={tenantContext?.organizations ?? []}
      workspaces={tenantContext?.workspaces ?? []}
      workspaceId={tenantContext?.workspaceId}
      projects={projectOptions}
      currentProjectId={projectId ?? undefined}
      isSuperAdmin={isSuperAdmin}
      canManageAdmin={canManageAdmin}
    >
      {children}
    </AppShell>
  );
}
