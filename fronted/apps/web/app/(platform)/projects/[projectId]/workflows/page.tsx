import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { CreateWorkflowForm } from "@/modules/workflow/components/WorkflowPanel";
import { WorkflowList } from "@/modules/workflow/components/WorkflowList";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";
import { loadWorkflows } from "@/server/loaders/workflows";

interface ProjectWorkflowsPageProps {
  params: Promise<{ projectId: string }>;
}

export default async function ProjectWorkflowsPage({ params }: ProjectWorkflowsPageProps) {
  const session = await loadSession();
  if (!session) redirect("/login");

  const { projectId } = await params;
  const cookieStore = await cookies();
  const workspaces = await loadWorkspaces();
  const workspaceId = cookieStore.get(WORKSPACE_COOKIE)?.value ?? workspaces[0]?.id;
  if (!workspaceId) redirect("/dashboard");

  let project;
  try {
    project = await loadProject(workspaceId, projectId);
  } catch {
    notFound();
  }

  const workflows = await loadWorkflows(workspaceId, projectId);

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow={project.name}
        title="工作流"
        description="编排项目自动化流程，管理 Workflow 定义与运行。"
      />
      <CreateWorkflowForm workspaceId={workspaceId} projectId={projectId} />
      <WorkflowList workflows={workflows} projectId={projectId} />
    </div>
  );
}
