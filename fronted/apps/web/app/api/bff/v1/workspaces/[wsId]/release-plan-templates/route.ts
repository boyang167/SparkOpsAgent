import { createBffHandler } from "@sparkops/bff-core";
import { listReleasePlanTemplates } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const items = await listReleasePlanTemplates(ctx, wsId);
  return { items };
});
