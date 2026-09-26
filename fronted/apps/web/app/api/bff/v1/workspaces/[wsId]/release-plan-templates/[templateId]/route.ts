import { createBffHandler } from "@sparkops/bff-core";
import { deleteReleasePlanTemplate } from "@sparkops/bff-modules/product";

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  return deleteReleasePlanTemplate(ctx, wsId, templateId);
});
