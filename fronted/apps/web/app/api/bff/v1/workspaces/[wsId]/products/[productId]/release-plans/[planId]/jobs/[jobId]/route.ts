import { createBffHandler } from "@sparkops/bff-core";
import { deleteReleasePlanJob, updateReleasePlanJob } from "@sparkops/bff-modules/product";
import type { UpdateReleasePlanJobCommand } from "@sparkops/contracts/product";

export const PATCH = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  const jobId = params?.jobId;
  if (!wsId || !productId || !planId || !jobId) throw new Error("workspace, product, plan and job required");
  const input = (await req.json()) as UpdateReleasePlanJobCommand;
  return updateReleasePlanJob(ctx, wsId, productId, planId, jobId, input);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  const jobId = params?.jobId;
  if (!wsId || !productId || !planId || !jobId) throw new Error("workspace, product, plan and job required");
  return deleteReleasePlanJob(ctx, wsId, productId, planId, jobId);
});
