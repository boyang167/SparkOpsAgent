import { createBffHandler } from "@sparkops/bff-core";
import { listWorkItemExecutions } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const workItemId = params?.workItemId;
  if (!wsId || !productId || !workItemId) throw new Error("workspace, product and work item required");
  const items = await listWorkItemExecutions(ctx, wsId, productId, workItemId);
  return { items };
});
