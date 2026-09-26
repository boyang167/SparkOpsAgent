import { createBffHandler } from "@sparkops/bff-core";
import { rejectReleasePlan } from "@sparkops/bff-modules/product";
import type { RejectReleasePlanCommand } from "@sparkops/contracts/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  const input = (await req.json().catch(() => ({}))) as RejectReleasePlanCommand;
  return rejectReleasePlan(ctx, wsId, productId, planId, input);
});
