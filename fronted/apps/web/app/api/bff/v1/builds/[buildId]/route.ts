import { createBffHandler } from "@sparkops/bff-core";
import { getBuild } from "@sparkops/bff-modules/delivery";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const buildId = params?.buildId;
  const wsId = ctx.workspaceId;
  if (!buildId || !wsId) {
    throw new Error("build id and workspace required");
  }
  return getBuild(ctx, wsId, buildId);
});
