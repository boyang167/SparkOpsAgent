import { createBffHandler } from "@sparkops/bff-core";
import { continueProductWorkflowRun } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  const runId = params?.runId;
  if (!wsId || !productId || !workflowId || !runId) {
    throw new Error("workspace, product, workflow and run required");
  }
  return continueProductWorkflowRun(ctx, wsId, productId, workflowId, runId);
});
