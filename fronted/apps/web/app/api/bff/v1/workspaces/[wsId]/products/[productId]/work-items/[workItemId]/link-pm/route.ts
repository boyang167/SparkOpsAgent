import { createBffHandler } from "@sparkops/bff-core";
import { linkPmIssue } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workItemId = params?.workItemId;
  if (!wsId || !productId || !workItemId) throw new Error("workspace, product and work item required");
  const body = await req.json();
  return linkPmIssue(ctx, wsId, productId, workItemId, body);
});
