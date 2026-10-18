import { createBffHandler } from "@sparkops/bff-core";
import { listMcpTools } from "@sparkops/bff-modules/mcp";

export const GET = createBffHandler(async (ctx, _req) => {
  const workspaceId = ctx.workspaceId;
  if (!workspaceId) {
    throw new Error("workspace required");
  }
  const items = await listMcpTools(ctx, workspaceId);
  return { items };
});
