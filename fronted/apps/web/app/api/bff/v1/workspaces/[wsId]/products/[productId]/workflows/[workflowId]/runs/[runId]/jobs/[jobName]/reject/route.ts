import { createBffHandler } from "@sparkops/bff-core";
import { rejectWorkflowJob } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  const runId = params?.runId;
  const jobName = params?.jobName;
  if (!wsId || !productId || !workflowId || !runId || !jobName) {
    throw new Error("workspace, product, workflow, run and job required");
  }
  const body = (await req.json().catch(() => ({}))) as import("@sparkops/contracts/product").RejectWorkflowJobCommand;
  return rejectWorkflowJob(ctx, wsId, productId, workflowId, runId, decodeURIComponent(jobName), body);
});
