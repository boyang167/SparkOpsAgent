import { createBffHandler } from "@sparkops/bff-core";
import { listProductWorkflowRuns, startProductWorkflowRun } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  if (!wsId || !productId || !workflowId) throw new Error("workspace, product and workflow required");
  const items = await listProductWorkflowRuns(ctx, wsId, productId, workflowId);
  return { items };
});

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  if (!wsId || !productId || !workflowId) throw new Error("workspace, product and workflow required");
  return startProductWorkflowRun(ctx, wsId, productId, workflowId);
});
