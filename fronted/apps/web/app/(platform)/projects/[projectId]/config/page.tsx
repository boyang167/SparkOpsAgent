import { Suspense } from "react";
import { notFound, redirect } from "next/navigation";
import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import { ProjectConfigPageClient } from "@/modules/runtime/components/ProjectConfigPageClient";
import { loadSession } from "@/server/loaders/session";
import { loadWorkspaces } from "@/server/loaders/workspaces";
import { loadProject } from "@/server/loaders/projects";

interface ProjectConfigPageProps {
  params: Promise<{ projectId: string }>;
}

export default async function ProjectConfigPage({ params }: ProjectConfigPageProps) {
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
    <Suspense fallback={<div className="text-muted p-4">加载配置…</div>}>
      <ProjectConfigPageClient workspaceId={workspaceId} project={project} />
    </Suspense>
  );
}
