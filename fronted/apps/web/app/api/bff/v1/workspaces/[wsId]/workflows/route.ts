import { createBffHandler } from "@sparkops/bff-core";
import { listWorkspaceWorkflows } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const items = await listWorkspaceWorkflows(ctx, wsId);
  return { items };
});
