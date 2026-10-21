import { createBffHandler } from "@sparkops/bff-core";
import { uninstallPlugin } from "@sparkops/bff-modules/plugins";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const pluginId = params?.id;
  if (!pluginId) throw new Error("plugin required");
  return uninstallPlugin(ctx, pluginId);
});
