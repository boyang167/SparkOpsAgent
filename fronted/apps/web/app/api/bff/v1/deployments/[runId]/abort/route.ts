import { createBffHandler } from "@sparkops/bff-core";
import { abortDeployment } from "@sparkops/bff-modules/delivery";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const runId = params?.runId;
  const wsId = ctx.workspaceId;
  if (!runId || !wsId) throw new Error("deployment id and workspace required");
  return abortDeployment(ctx, wsId, runId);
});
