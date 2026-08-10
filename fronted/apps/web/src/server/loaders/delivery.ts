import { listBuilds, getBuild } from "@sparkops/bff-modules/delivery";
import { listPods } from "@sparkops/bff-modules/cluster";
import { loadBffContext } from "./context";

export async function loadBuilds(workspaceId: string, projectId: string) {
  const ctx = await loadBffContext();
  return listBuilds(ctx, workspaceId, projectId);
}

export async function loadBuild(workspaceId: string, buildId: string) {
  const ctx = await loadBffContext();
  return getBuild(ctx, workspaceId, buildId);
}

export async function loadPods(workspaceId: string, projectId: string) {
  const ctx = await loadBffContext();
  return listPods(ctx, workspaceId, projectId);
}
