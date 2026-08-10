import { createBffHandler } from "@sparkops/bff-core";
import { abortBuild } from "@sparkops/bff-modules/delivery";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const buildId = params?.buildId;
  const wsId = ctx.workspaceId;
  if (!buildId || !wsId) {
    throw new Error("build id and workspace required");
  }
  return abortBuild(ctx, wsId, buildId);
});
