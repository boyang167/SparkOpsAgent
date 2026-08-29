import { createBffHandler } from "@sparkops/bff-core";
import { revokeWorkspaceCluster } from "@sparkops/bff-modules/platform";

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  const clusterId = params?.clusterId;
  if (!wsId || !clusterId) throw new Error("workspace and cluster required");
  return revokeWorkspaceCluster(ctx, wsId, clusterId);
});
