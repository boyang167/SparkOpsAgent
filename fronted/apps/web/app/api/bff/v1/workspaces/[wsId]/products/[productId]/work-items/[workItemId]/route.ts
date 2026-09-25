import { createBffHandler } from "@sparkops/bff-core";
import { deleteWorkItem, getWorkItem, updateWorkItem } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workItemId = params?.workItemId;
  if (!wsId || !productId || !workItemId) throw new Error("workspace, product and work item required");
  return getWorkItem(ctx, wsId, productId, workItemId);
});

export const PATCH = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workItemId = params?.workItemId;
  if (!wsId || !productId || !workItemId) throw new Error("workspace, product and work item required");
  const body = await req.json();
  return updateWorkItem(ctx, wsId, productId, workItemId, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workItemId = params?.workItemId;
  if (!wsId || !productId || !workItemId) throw new Error("workspace, product and work item required");
  await deleteWorkItem(ctx, wsId, productId, workItemId);
  return { deleted: true };
});
