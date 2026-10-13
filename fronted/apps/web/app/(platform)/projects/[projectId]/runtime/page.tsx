import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { PageHeader } from "@/components/ui/PageHeader";
import { WorkspaceClusterPanel } from "@/modules/infra/components/WorkspaceClusterPanel";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";
import { projectDisplayName } from "@/modules/workspace/components/project/project-utils";

interface ProjectRuntimePageProps {
  params: Promise<{ projectId: string }>;
}

export default async function ProjectRuntimePage({ params }: ProjectRuntimePageProps) {
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

  return (
    <div className="stack-lg">
      <PageHeader
        eyebrow={projectDisplayName(project)}
        title="运行环境"
        description="首个集群负责构建与部署，其余集群自动同步。"
      />
      <WorkspaceClusterPanel workspaceId={workspaceId} />
    </div>
  );
}
