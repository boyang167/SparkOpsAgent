import { createBffHandler } from "@sparkops/bff-core";
import { getDeploymentOptions } from "@sparkops/bff-modules/delivery";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) throw new Error("project id and workspace required");
  return getDeploymentOptions(ctx, wsId, projectId);
});
