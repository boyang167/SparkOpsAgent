import { createBffHandler } from "@sparkops/bff-core";
import { putDeploymentResources } from "@sparkops/bff-modules/cluster";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  const body = await req.json();
  return putDeploymentResources(ctx, wsId, projectId, body as Record<string, unknown>);
});
