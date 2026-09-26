import { createBffHandler } from "@sparkops/bff-core";
import { createReleasePlanFromTemplate } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  const body = await req.json();
  return createReleasePlanFromTemplate(ctx, wsId, templateId, body);
});
