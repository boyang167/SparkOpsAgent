import { createBffHandler } from "@sparkops/bff-core";
import {
  deleteProductWorkflow,
  getProductWorkflow,
  updateProductWorkflow,
} from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  if (!wsId || !productId || !workflowId) throw new Error("workspace, product and workflow required");
  return getProductWorkflow(ctx, wsId, productId, workflowId);
});

export const PATCH = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  if (!wsId || !productId || !workflowId) throw new Error("workspace, product and workflow required");
  const body = await req.json();
  return updateProductWorkflow(ctx, wsId, productId, workflowId, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workflowId = params?.workflowId;
  if (!wsId || !productId || !workflowId) throw new Error("workspace, product and workflow required");
  return deleteProductWorkflow(ctx, wsId, productId, workflowId);
});
