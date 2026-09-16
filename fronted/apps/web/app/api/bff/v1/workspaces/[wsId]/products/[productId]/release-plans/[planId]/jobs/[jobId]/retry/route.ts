import { createBffHandler } from "@sparkops/bff-core";
import { retryReleasePlanJob } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  const jobId = params?.jobId;
  if (!wsId || !productId || !planId || !jobId) throw new Error("missing params");
  return retryReleasePlanJob(ctx, wsId, productId, planId, jobId);
});
