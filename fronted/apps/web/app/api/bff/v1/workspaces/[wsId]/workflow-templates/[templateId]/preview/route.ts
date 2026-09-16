import { createBffHandler } from "@sparkops/bff-core";
import { previewWorkflowTemplate } from "@sparkops/bff-modules/product";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  const body = (await req.json()) as { key_vals?: Record<string, string> };
  return previewWorkflowTemplate(ctx, wsId, templateId, body.key_vals ?? {});
});
