import { createBffHandler } from "@sparkops/bff-core";
import { getReleasePlan, updateReleasePlan } from "@sparkops/bff-modules/product";
import type { UpdateReleasePlanCommand } from "@sparkops/contracts/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  return getReleasePlan(ctx, wsId, productId, planId);
});

export const PATCH = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  const input = (await req.json()) as UpdateReleasePlanCommand;
  return updateReleasePlan(ctx, wsId, productId, planId, input);
});
