import { createBffHandler } from "@sparkops/bff-core";
import { approveWorkflowJob } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  const runId = params?.runId;
  const jobName = params?.jobName;
  if (!wsId || !productId || !workflowId || !runId || !jobName) {
    throw new Error("workspace, product, workflow, run and job required");
  }
  return approveWorkflowJob(ctx, wsId, productId, workflowId, runId, decodeURIComponent(jobName));
});
