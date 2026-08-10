import type { BffContext } from "@sparkops/bff-core";

export async function getMyPermissions(ctx: BffContext, workspaceId: string): Promise<string[]> {
  const data = await ctx.upstream.governance.myPermissions(workspaceId);
  return data.permissions;
}
