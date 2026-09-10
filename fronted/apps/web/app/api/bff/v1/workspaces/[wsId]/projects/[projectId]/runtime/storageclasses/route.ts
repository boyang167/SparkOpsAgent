import { createBffHandler } from "@sparkops/bff-core";
import { listStorageClasses } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  return listStorageClasses(ctx, wsId, projectId);
});
