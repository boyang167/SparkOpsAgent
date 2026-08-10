import Link from "next/link";

import { redirect } from "next/navigation";

import { PageHeader } from "@/components/ui/PageHeader";

import { CreateWorkspaceForm } from "@/modules/tenant/components/CreateWorkspaceForm";

import { TenantSetupPanel } from "@/modules/tenant/components/TenantSetupPanel";

import { ProjectList } from "@/modules/workspace/components/ProjectList";

import { CreateProjectForm } from "@/modules/workspace/components/CreateProjectForm";

import { loadOrganizations } from "@/server/loaders/organizations";
import { loadSession } from "@/server/loaders/session";
import { loadResolvedWorkspaceId } from "@/server/loaders/workspace-context";
import { loadProjects } from "@/server/loaders/projects";



export default async function DashboardPage() {

  const session = await loadSession();

  if (!session) {

    redirect("/login");

  }



  const organizations = await loadOrganizations();
  const { workspaces, workspaceId } = await loadResolvedWorkspaceId();
  const projects = workspaceId ? await loadProjects(workspaceId) : [];



  return (

    <div className="stack-lg">

      <PageHeader

        eyebrow="Dashboard"

        title={`你好，${session.display_name}`}

        description="在当前 Workspace 下管理项目与 Agent。"

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

        <div className="section-header">

          <div>

            <h2 className="section-title">项目列表</h2>

            <p className="text-muted mt-1">浏览项目并进入 Agent 对话</p>

          </div>

          {workspaceId ? (

            <div className="flex flex-wrap items-center gap-2">

              <CreateWorkspaceForm organizations={organizations} workspaces={workspaces} />

              <CreateProjectForm workspaceId={workspaceId} />

              <Link href="/settings/tenant" className="btn-ghost text-sm">

                组织管理

              </Link>

            </div>

          ) : null}

        </div>

        {!workspaceId ? (

          <TenantSetupPanel organizations={organizations} workspaces={workspaces} />

        ) : (

          <ProjectList projects={projects} workspaceId={workspaceId} />

        )}

      </section>

    </div>

  );

}

