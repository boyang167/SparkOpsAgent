import { createBffHandler } from "@sparkops/bff-core";
import { executeReleasePlan } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("missing params");
  return executeReleasePlan(ctx, wsId, productId, planId);
});
