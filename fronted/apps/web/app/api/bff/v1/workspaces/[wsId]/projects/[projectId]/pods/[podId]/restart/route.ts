import { createBffHandler } from "@sparkops/bff-core";
import { restartPod } from "@sparkops/bff-modules/cluster";

export const POST = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const podId = params?.podId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !podId || !wsId) {
    throw new Error("project id, pod id and workspace required");
  }
  return restartPod(ctx, wsId, projectId, podId);
});
