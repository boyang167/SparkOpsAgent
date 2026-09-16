import { createBffHandler } from "@sparkops/bff-core";
import { deleteEnvironment, updateEnvironment } from "@sparkops/bff-modules/product";

export const PATCH = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const envId = params?.envId;
  if (!wsId || !productId || !envId) throw new Error("workspace, product and environment required");
  const body = await req.json();
  return updateEnvironment(ctx, wsId, productId, envId, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const envId = params?.envId;
  if (!wsId || !productId || !envId) throw new Error("workspace, product and environment required");
  return deleteEnvironment(ctx, wsId, productId, envId);
});
