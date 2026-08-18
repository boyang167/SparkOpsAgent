import { redirect } from "next/navigation";
import { PageHeader } from "@/components/ui/PageHeader";
import { TenantSetupPanel } from "@/modules/tenant/components/TenantSetupPanel";
import { ProjectPanel } from "@/modules/workspace/components/ProjectPanel";
import { loadOrganizations } from "@/server/loaders/organizations";
import { loadSession } from "@/server/loaders/session";
import { loadResolvedWorkspaceId } from "@/server/loaders/workspace-context";
import { loadProjects } from "@/server/loaders/projects";

export default async function DashboardPage({
  searchParams,
}: {
  searchParams: Promise<{ create?: string }>;
}) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const organizations = await loadOrganizations();
  const { workspaces, workspaceId } = await loadResolvedWorkspaceId();
  const projects = workspaceId ? await loadProjects(workspaceId) : [];
  const params = await searchParams;

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow="项目"
        title="项目管理"
        description="管理当前 Workspace 下的项目，创建、配置并进入项目工作台"
      />

      <div className="grid gap-4 sm:grid-cols-3">
        <div className="stat-card">
          <p className="text-muted text-sm">项目</p>
          <p className="text-2xl font-semibold mt-1">{projects.length}</p>
        </div>
        <div className="stat-card">
          <p className="text-muted text-sm">Workspace</p>
          <p className="text-2xl font-semibold mt-1">{workspaces.length}</p>
        </div>
        <div className="stat-card">
          <p className="text-muted text-sm">账号</p>
          <p className="text-sm font-medium mt-2 truncate">{session.email}</p>
        </div>
      </div>

      <section className="section">
        {!workspaceId ? (
          <>
            <div className="section-header">
              <div>
                <h2 className="section-title">项目列表</h2>
                <p className="text-muted mt-1">请先完成 Workspace 初始化</p>
              </div>
            </div>
            <TenantSetupPanel organizations={organizations} workspaces={workspaces} />
          </>
        ) : (
          <ProjectPanel
            workspaceId={workspaceId}
            initialCreateOpen={params.create === "1"}
            createQueryPath="/dashboard"
          />
        )}
      </section>
    </div>
  );
}
