import { cookies } from "next/headers";
import { AUTH_COOKIE, WORKSPACE_COOKIE, createBffContextFromToken } from "@sparkops/bff-core";
import type { BffContext } from "@sparkops/bff-core";

export async function loadBffContext(workspaceIdOverride?: string): Promise<BffContext> {
  const cookieStore = await cookies();
  const token = cookieStore.get(AUTH_COOKIE)?.value;
  const workspaceId =
    workspaceIdOverride ?? cookieStore.get(WORKSPACE_COOKIE)?.value;
  return createBffContextFromToken(token, workspaceId);
}

export async function loadWorkspaceId(): Promise<string | undefined> {
  const cookieStore = await cookies();
  return cookieStore.get(WORKSPACE_COOKIE)?.value;
}
