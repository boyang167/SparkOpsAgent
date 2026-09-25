import { createBffHandler } from "@sparkops/bff-core";
import { deleteWorkspaceSprintProcessTemplate } from "@sparkops/bff-modules/product";

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const templateId = params?.templateId;
  if (!wsId || !templateId) throw new Error("workspace and template required");
  await deleteWorkspaceSprintProcessTemplate(ctx, wsId, templateId);
  return { deleted: true };
});
