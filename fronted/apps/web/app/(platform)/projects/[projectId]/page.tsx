import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { AgentList } from "@/modules/agent/components/AgentList";
import { CreateAgentForm } from "@/modules/agent/components/CreateAgentForm";
import { CreateWorkflowForm, WorkflowSettings } from "@/modules/workflow/components/WorkflowPanel";
import { WorkflowList } from "@/modules/workflow/components/WorkflowList";
import { EditProjectForm, DeleteProjectButton } from "@/modules/workspace/components/ProjectSettings";
import { ProjectMembersPanel } from "@/modules/workspace/components/ProjectMembersPanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";
import { loadAgents } from "@/server/loaders/agents";
import { loadWorkflows } from "@/server/loaders/workflows";

interface ProjectPageProps {
  params: Promise<{ projectId: string }>;
}

export default async function ProjectPage({ params }: ProjectPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { projectId } = await params;
  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId =
    cookieStore.get(WORKSPACE_COOKIE)?.value ??
    (await loadWorkspaceId()) ??
    workspaces[0]?.id;

  if (!workspaceId) {
    redirect("/dashboard");
  }

  let project;
  try {
    project = await loadProject(workspaceId, projectId);
  } catch {
    notFound();
  }

  const agents = await loadAgents(workspaceId, projectId);
  const workflows = await loadWorkflows(workspaceId, projectId);

  return (
    <div className="stack-lg">
      <Link href="/dashboard" className="link-back">
        ← 返回项目列表
      </Link>

      <PageHeader
        eyebrow="Project"
        title={project.name}
        description={project.description || undefined}
        actions={
          <div className="flex flex-wrap items-center gap-2">
            <span className="badge badge-neutral">{project.status}</span>
            <EditProjectForm workspaceId={workspaceId} project={project} />
            <DeleteProjectButton
              workspaceId={workspaceId}
              projectId={projectId}
              projectName={project.name}
            />
          </div>
        }
      />

      <ProjectMembersPanel workspaceId={workspaceId} projectId={projectId} />

      <section className="section">
        <div className="section-header">
          <div>
            <h2 className="section-title">Agents</h2>
            <p className="text-muted mt-1">创建并发布 Agent，进入对话调试</p>
          </div>
          <CreateAgentForm workspaceId={workspaceId} projectId={projectId} />
        </div>
        <AgentList agents={agents} projectId={projectId} />
      </section>

      <section className="section">
        <div className="section-header">
          <div>
            <h2 className="section-title">Workflows</h2>
            <p className="text-muted mt-1">编排项目内的自动化流程</p>
          </div>
          <CreateWorkflowForm workspaceId={workspaceId} projectId={projectId} />
        </div>
        <WorkflowList workflows={workflows} projectId={projectId} />
        <WorkflowSettings workspaceId={workspaceId} workflows={workflows} />
      </section>
    </div>
  );
}
