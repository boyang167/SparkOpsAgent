import { createBffHandler } from "@sparkops/bff-core";
import { createWorkflowTemplate, listWorkflowTemplates } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const items = await listWorkflowTemplates(ctx, wsId);
  return { items };
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const body = (await req.json()) as import("@sparkops/contracts/product").CreateWorkflowTemplateCommand;
  return createWorkflowTemplate(ctx, wsId, body);
});
