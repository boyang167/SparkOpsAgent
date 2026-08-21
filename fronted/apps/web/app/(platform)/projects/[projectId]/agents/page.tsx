import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { AgentList } from "@/modules/agent/components/AgentList";
import { CreateAgentForm } from "@/modules/agent/components/CreateAgentForm";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";
import { loadAgents } from "@/server/loaders/agents";

interface ProjectAgentsPageProps {
  params: Promise<{ projectId: string }>;
}

export default async function ProjectAgentsPage({ params }: ProjectAgentsPageProps) {
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

  const agents = await loadAgents(workspaceId, projectId);

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow={project.name}
        title="智能体"
        description="管理项目内的 Agent，点击进入对话与配置。"
      />
      <CreateAgentForm workspaceId={workspaceId} projectId={projectId} />
      <AgentList agents={agents} projectId={projectId} />
    </div>
  );
}
