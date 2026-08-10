import { redirect } from "next/navigation";
import { loadSession } from "@/server/loaders/session";
import { loadResolvedWorkspaceId } from "@/server/loaders/workspace-context";
import { loadProjects } from "@/server/loaders/projects";
import type { ProjectVM } from "@sparkops/contracts/workspace";

export interface PlatformPageContext {
  workspaceId: string;
  projects: ProjectVM[];
}

export async function loadPlatformPageContext(): Promise<PlatformPageContext> {
  const session = await loadSession();
  if (!session) {
    redirect("/login");
  }

  const { workspaceId } = await loadResolvedWorkspaceId();

  if (!workspaceId) {
    redirect("/dashboard");
  }

  const projects = await loadProjects(workspaceId);
  return { workspaceId, projects };
}
