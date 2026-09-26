import { createBffHandler } from "@sparkops/bff-core";
import {
  deleteWorkspaceSprintProcessTemplate,
  updateWorkspaceSprintProcessTemplate,
} from "@sparkops/bff-modules/product";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  const body = await req.json();
  return updateWorkspaceSprintProcessTemplate(ctx, wsId, templateId, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  await deleteWorkspaceSprintProcessTemplate(ctx, wsId, templateId);
  return { deleted: true };
});
