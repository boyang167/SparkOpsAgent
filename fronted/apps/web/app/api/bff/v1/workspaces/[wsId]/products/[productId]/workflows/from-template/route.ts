import { createBffHandler } from "@sparkops/bff-core";
import { createProductWorkflowFromTemplate } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  if (!wsId || !productId) throw new Error("workspace and product required");
  const body = await req.json();
  return createProductWorkflowFromTemplate(ctx, wsId, productId, body);
});
