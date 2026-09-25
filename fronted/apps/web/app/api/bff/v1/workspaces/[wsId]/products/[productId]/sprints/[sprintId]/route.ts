import { createBffHandler } from "@sparkops/bff-core";
import { updateSprint } from "@sparkops/bff-modules/product";

export const PATCH = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const productId = params?.productId;
  const sprintId = params?.sprintId;
  if (!wsId || !productId || !sprintId) throw new Error("workspace, product and sprint required");
  const body = await req.json();
  return updateSprint(ctx, wsId, productId, sprintId, body);
});
