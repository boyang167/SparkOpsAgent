import { createBffHandler } from "@sparkops/bff-core";
import { rejectRequirement } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const reqId = params?.reqId;
  if (!wsId || !productId || !reqId) throw new Error("workspace, product and requirement required");
  const body = (await req.json().catch(() => ({}))) as { reason?: string };
  return rejectRequirement(ctx, wsId, productId, reqId, body.reason);
});
