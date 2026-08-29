import { createBffHandler } from "@sparkops/bff-core";
import { getDeployment } from "@sparkops/bff-modules/delivery";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const runId = params?.runId;
  const wsId = ctx.workspaceId;
  if (!runId || !wsId) throw new Error("deployment id and workspace required");
  return getDeployment(ctx, wsId, runId);
});
