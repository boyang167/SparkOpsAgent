import { createBffHandler } from "@sparkops/bff-core";
import { enableWorkspacePlugin } from "@sparkops/bff-modules/plugins";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const workspaceId = params?.wsId;
  const pluginId = params?.id;
  if (!workspaceId) throw new Error("workspace required");
  if (!pluginId) throw new Error("plugin required");
  return enableWorkspacePlugin(ctx, workspaceId, pluginId);
});
