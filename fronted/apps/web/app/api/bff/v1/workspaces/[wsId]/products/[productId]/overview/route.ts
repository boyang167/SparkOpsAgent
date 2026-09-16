import { createBffHandler } from "@sparkops/bff-core";
import { getProductOverview } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  return getProductOverview(ctx, wsId, productId);
});
