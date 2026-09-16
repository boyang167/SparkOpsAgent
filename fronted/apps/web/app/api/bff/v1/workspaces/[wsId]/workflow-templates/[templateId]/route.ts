import { createBffHandler } from "@sparkops/bff-core";
import { getWorkflowTemplate, updateWorkflowTemplate } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  return getWorkflowTemplate(ctx, wsId, templateId);
});

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  const body = (await req.json()) as import("@sparkops/contracts/product").UpdateWorkflowTemplateCommand;
  return updateWorkflowTemplate(ctx, wsId, templateId, body);
});
