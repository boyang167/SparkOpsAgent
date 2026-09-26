import { createBffHandler } from "@sparkops/bff-core";
import { copyReleasePlan } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  const body = (await req.json().catch(() => ({}))) as { name?: string };
  return copyReleasePlan(ctx, wsId, productId, planId, body);
});
