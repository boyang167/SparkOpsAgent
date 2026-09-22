import { createBffHandler } from "@sparkops/bff-core";
import { cancelProductWorkflowRun } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  const runId = params?.runId;
  if (!wsId || !productId || !workflowId || !runId) {
    throw new Error("workspace, product, workflow and run required");
  }
  const body = (await req.json().catch(() => ({}))) as { message?: string };
  return cancelProductWorkflowRun(ctx, wsId, productId, workflowId, runId, body);
});
