import { createBffHandler } from "@sparkops/bff-core";
import { getSprintMetrics } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const sprintId = params?.sprintId;
  if (!wsId || !productId || !sprintId) throw new Error("workspace, product and sprint required");
  return getSprintMetrics(ctx, wsId, productId, sprintId);
});
