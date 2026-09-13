import { redirect } from "next/navigation";
import { WorkplaceDashboard } from "@/modules/dashboard/components/WorkplaceDashboard";
import { loadProjects } from "@/server/loaders/projects";
import { loadSession } from "@/server/loaders/session";
import { loadResolvedWorkspaceId } from "@/server/loaders/workspace-context";

export default async function WorkplacePage() {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { workspaces, workspaceId } = await loadResolvedWorkspaceId();
  if (!workspaceId) {
    redirect("/dashboard");
  }

  const projects = await loadProjects(workspaceId);
  const currentWorkspace = workspaces.find((workspace) => workspace.id === workspaceId);

  return (
    <WorkplaceDashboard
      session={session}
      workspaces={workspaces}
      workspaceId={workspaceId}
      workspaceName={currentWorkspace?.name ?? "Workspace"}
      projects={projects}
    />
  );
}
