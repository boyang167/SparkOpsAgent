import { createBffHandler } from "@sparkops/bff-core";
import { listReleasePlanVersions } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const planId = params?.planId;
  if (!wsId || !productId || !planId) throw new Error("workspace, product and plan required");
  const offset = Number(new URL(req.url).searchParams.get("offset") ?? "0");
  if (!Number.isInteger(offset) || offset < 0) throw new Error("invalid offset");
  return listReleasePlanVersions(ctx, wsId, productId, planId, offset);
});
