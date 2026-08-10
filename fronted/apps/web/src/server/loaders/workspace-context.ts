import { cookies } from "next/headers";
import { WORKSPACE_COOKIE } from "@sparkops/bff-core";
import type { WorkspaceVM } from "@sparkops/contracts/tenant";
import { loadWorkspaces } from "./workspaces";

export function resolveWorkspaceId(
  workspaces: WorkspaceVM[],
  cookieWorkspaceId?: string,
): string | undefined {
  if (cookieWorkspaceId && workspaces.some((ws) => ws.id === cookieWorkspaceId)) {
    return cookieWorkspaceId;
  }
  return workspaces[0]?.id;
}

export async function loadResolvedWorkspaceId(): Promise<{
  workspaces: WorkspaceVM[];
  workspaceId: string | undefined;
}> {
  const workspaces = await loadWorkspaces();
  const cookieStore = await cookies();
  const workspaceId = resolveWorkspaceId(
    workspaces,
    cookieStore.get(WORKSPACE_COOKIE)?.value,
  );
  return { workspaces, workspaceId };
}
