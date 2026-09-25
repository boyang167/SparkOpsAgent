import { createBffHandler } from "@sparkops/bff-core";
import { getSprintProcess, upsertSprintProcess } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  return getSprintProcess(ctx, wsId, productId);
});

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  return upsertSprintProcess(ctx, wsId, productId, body);
});
