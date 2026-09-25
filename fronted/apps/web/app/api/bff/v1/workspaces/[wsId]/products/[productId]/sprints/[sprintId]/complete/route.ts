import { createBffHandler } from "@sparkops/bff-core";
import { completeSprint } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const sprintId = params?.sprintId;
  if (!wsId || !productId || !sprintId) throw new Error("workspace, product and sprint required");
  return completeSprint(ctx, wsId, productId, sprintId);
});
