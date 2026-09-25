import { createBffHandler } from "@sparkops/bff-core";
import { deleteSprintProcess, updateSprintProcessById } from "@sparkops/bff-modules/product";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const processId = params?.processId;
  if (!wsId || !productId || !processId) throw new Error("workspace, product and process required");
  const body = await req.json();
  return updateSprintProcessById(ctx, wsId, productId, processId, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const processId = params?.processId;
  if (!wsId || !productId || !processId) throw new Error("workspace, product and process required");
  await deleteSprintProcess(ctx, wsId, productId, processId);
  return { deleted: true };
});
