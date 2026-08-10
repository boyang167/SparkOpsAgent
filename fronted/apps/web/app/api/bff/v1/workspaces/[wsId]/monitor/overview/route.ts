import { createBffHandler } from "@sparkops/bff-core";
import { getMonitorOverview } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) {
    throw new Error("workspace required");
  }
  return getMonitorOverview(ctx, wsId);
});
