import { createBffHandler } from "@sparkops/bff-core";
import { publishWorkflowTemplate } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  return publishWorkflowTemplate(ctx, wsId, templateId);
});
