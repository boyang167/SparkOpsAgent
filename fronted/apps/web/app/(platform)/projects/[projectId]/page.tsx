import { Suspense } from "react";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { ProjectWorkbenchPage } from "@/modules/workspace/components/project/ProjectWorkbenchPage";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaceId } from "@/server/loaders/context";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";

interface ProjectPageProps {
  params: Promise<{ projectId: string }>;
  searchParams: Promise<{ configure?: string; release?: string; tab?: string }>;
}

export default async function ProjectPage({ params, searchParams }: ProjectPageProps) {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { projectId } = await params;
  const query = await searchParams;
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

  return (
    <Suspense fallback={<div className="text-muted p-4">加载项目详情…</div>}>
      <ProjectWorkbenchPage
        workspaceId={workspaceId}
        project={project}
        initialConfigure={query.configure === "1"}
      />
    </Suspense>
  );
}
