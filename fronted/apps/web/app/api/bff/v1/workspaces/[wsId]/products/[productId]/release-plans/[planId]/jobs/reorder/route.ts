import { createBffHandler } from "@sparkops/bff-core";
import { reorderReleasePlanJobs } from "@sparkops/bff-modules/product";
import type { ReorderReleasePlanJobsCommand } from "@sparkops/contracts/product";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  const body = (await req.json()) as ReorderReleasePlanJobsCommand;
  return reorderReleasePlanJobs(ctx, wsId, productId, planId, body.job_ids);
});
