import { createBffHandler } from "@sparkops/bff-core";
import { getWorkflowRun } from "@sparkops/bff-modules/workflow";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const runId = params?.runId;
  const wsId = ctx.workspaceId;
  if (!runId || !wsId) {
    throw new Error("run id and workspace required");
  }
  return getWorkflowRun(ctx, wsId, runId);
});
