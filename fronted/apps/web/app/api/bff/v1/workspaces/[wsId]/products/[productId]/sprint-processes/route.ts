import { createBffHandler } from "@sparkops/bff-core";
import { createSprintProcess, listSprintProcesses } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const items = await listSprintProcesses(ctx, wsId, productId);
  return { items };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  return createSprintProcess(ctx, wsId, productId, body);
});
