import { createBffHandler } from "@sparkops/bff-core";
import { listPVCs } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  if (!projectId || !wsId) throw new Error("project id and workspace required");
  return listPVCs(ctx, wsId, projectId);
});
