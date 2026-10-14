import { createBffHandler } from "@sparkops/bff-core";
import { archiveSprint } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const sprintId = params?.sprintId;
  if (!wsId || !productId || !sprintId) throw new Error("workspace, product and sprint required");
  return archiveSprint(ctx, wsId, productId, sprintId);
});
