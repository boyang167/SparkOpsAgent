import { createBffHandler } from "@sparkops/bff-core";
import { releaseRequirement } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const reqId = params?.reqId;
  if (!wsId || !productId || !reqId) throw new Error("workspace, product and requirement required");
  return releaseRequirement(ctx, wsId, productId, reqId);
});
