import { createBffHandler } from "@sparkops/bff-core";
import { putServicePorts } from "@sparkops/bff-modules/cluster";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  const body = await req.json();
  return putServicePorts(ctx, wsId, projectId, body as Record<string, unknown>);
});
