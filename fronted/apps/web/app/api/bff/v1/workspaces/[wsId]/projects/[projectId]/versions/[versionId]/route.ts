import { createBffHandler } from "@sparkops/bff-core";
import { getProjectVersion } from "@sparkops/bff-modules/workspace";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const versionId = params?.versionId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !versionId || !wsId) {
    throw new Error("project id, version id and workspace required");
  }
  return getProjectVersion(ctx, wsId, projectId, versionId);
});
