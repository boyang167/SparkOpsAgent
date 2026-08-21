import { headers } from "next/headers";
import { AdminNav } from "@/components/layout/AdminNav";
import { AppShell } from "@/components/layout/AppShell";
import { extractProjectId } from "@/components/layout/context-utils";
import { PlatformNav } from "@/components/layout/PlatformNav";
import { PlatformOpsNav } from "@/components/layout/PlatformOpsNav";
import { ProjectNav } from "@/components/layout/ProjectNav";
import { resolveConsoleKind } from "@/components/layout/nav-tree";
import { loadConsoleAccess } from "@/server/loaders/console-access";
import { requirePathEntitlement } from "@/server/loaders/require-path-entitlement";
import { loadProject, loadProjects } from "@/server/loaders/projects";
import { loadTenantContext } from "@/server/loaders/tenant-context";

export default async function PlatformLayout({ children }: { children: React.ReactNode }) {
  const headerStore = await headers();
  const pathname = headerStore.get("x-pathname") ?? "";
  const consoleKind = resolveConsoleKind(pathname);
  const projectId = extractProjectId(pathname);

  const { session, isSuperAdmin, canManageAdmin } = await loadConsoleAccess();

  let tenantContext: Awaited<ReturnType<typeof loadTenantContext>> | null = null;
  let projectName: string | undefined;
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

    if (projectId && tenantContext?.workspaceId && consoleKind === "business") {
      try {
        const [project, projects] = await Promise.all([
          loadProject(tenantContext.workspaceId, projectId).catch(() => null),
          loadProjects(tenantContext.workspaceId).catch(() => []),
        ]);
        projectName = project?.name;
        projectOptions = projects.map((item) => ({ id: item.id, name: item.name }));
      } catch {
        // ignore project load errors
      }
    }
  }

  let nav =
    consoleKind === "admin" ? (
      <AdminNav />
    ) : consoleKind === "platform" ? (
      <PlatformOpsNav />
    ) : (
      <PlatformNav workspaceId={tenantContext?.workspaceId} />
    );

  if (projectId && projectName && consoleKind === "business") {
    nav = <ProjectNav projectId={projectId} projectName={projectName} />;
  }

  return (
    <AppShell
      nav={nav}
      session={session}
      organizations={tenantContext?.organizations ?? []}
      workspaces={tenantContext?.workspaces ?? []}
      workspaceId={tenantContext?.workspaceId}
      projects={projectOptions}
      currentProjectId={projectId ?? undefined}
      isSuperAdmin={isSuperAdmin}
      canManageAdmin={canManageAdmin}
      consoleKind={consoleKind}
    >
      {children}
    </AppShell>
  );
}
