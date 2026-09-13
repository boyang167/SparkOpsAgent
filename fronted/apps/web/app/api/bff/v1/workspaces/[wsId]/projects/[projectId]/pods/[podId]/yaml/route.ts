import { createBffHandler } from "@sparkops/bff-core";
import { getPodYAML } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const podId = params?.podId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !podId || !wsId) {
    throw new Error("project id, pod id and workspace required");
  }
  return getPodYAML(ctx, wsId, projectId, podId);
});
