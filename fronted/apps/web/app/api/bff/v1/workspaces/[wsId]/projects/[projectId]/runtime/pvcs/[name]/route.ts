import { createBffHandler } from "@sparkops/bff-core";
import { deletePVC } from "@sparkops/bff-modules/cluster";

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const name = params?.name;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId || !name) {
    throw new Error("project id, workspace and name required");
  }
  return deletePVC(ctx, wsId, projectId, name);
});
