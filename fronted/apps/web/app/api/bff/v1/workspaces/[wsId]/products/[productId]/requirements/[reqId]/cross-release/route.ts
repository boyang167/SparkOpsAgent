import { createBffHandler } from "@sparkops/bff-core";
import { crossReleaseRequirement } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const reqId = params?.reqId;
  if (!wsId || !productId || !reqId) throw new Error("workspace, product and requirement required");
  const body = (await req.json()) as { source_cluster: string; target_cluster: string };
  return crossReleaseRequirement(ctx, wsId, productId, reqId, body);
});
