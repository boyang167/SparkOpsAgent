import { createBffHandler } from "@sparkops/bff-core";
import { executeWorkItem } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workItemId = params?.workItemId;
  if (!wsId || !productId || !workItemId) throw new Error("workspace, product and work item required");
  let body = {};
  try {
    body = await req.json();
  } catch {
    body = {};
  }
  return executeWorkItem(ctx, wsId, productId, workItemId, body);
});
