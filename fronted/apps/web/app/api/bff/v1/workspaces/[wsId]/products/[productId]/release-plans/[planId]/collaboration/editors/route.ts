import { createBffHandler } from "@sparkops/bff-core";
import { listReleasePlanCollabEditors } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  return listReleasePlanCollabEditors(ctx, wsId, productId, planId);
});
