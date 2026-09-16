import { createBffHandler } from "@sparkops/bff-core";
import { getEnvironmentStatus } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const envId = params?.envId;
  if (!wsId || !productId || !envId) throw new Error("workspace, product and environment required");
  return getEnvironmentStatus(ctx, wsId, productId, envId);
});
