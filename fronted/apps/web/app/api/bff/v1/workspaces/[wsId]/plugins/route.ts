import { createBffHandler } from "@sparkops/bff-core";
import { listWorkspacePlugins } from "@sparkops/bff-modules/plugins";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const workspaceId = params?.wsId;
  if (!workspaceId) throw new Error("workspace required");
  return { items: await listWorkspacePlugins(ctx, workspaceId) };
});
