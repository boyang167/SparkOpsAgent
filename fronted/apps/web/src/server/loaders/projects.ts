import { listProjects, getProject } from "@sparkops/bff-modules/workspace";
import type { ProjectVM } from "@sparkops/contracts/workspace";
import { loadBffContext } from "./context";

export async function loadProjects(workspaceId: string): Promise<ProjectVM[]> {
  try {
    const ctx = await loadBffContext(workspaceId);
    return await listProjects(ctx, workspaceId);
  } catch {
    return [];
  }
}

export async function loadProject(workspaceId: string, projectId: string): Promise<ProjectVM> {
  const ctx = await loadBffContext(workspaceId);
  return getProject(ctx, workspaceId, projectId);
}
