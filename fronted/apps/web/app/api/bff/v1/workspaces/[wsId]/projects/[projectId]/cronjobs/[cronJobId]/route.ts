import { createBffHandler } from "@sparkops/bff-core";
import { deleteCronJob } from "@sparkops/bff-modules/delivery";

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  const projectId = params?.projectId;
  const cronJobId = params?.cronJobId;
  if (!wsId || !projectId || !cronJobId) throw new Error("missing params");
  return deleteCronJob(ctx, wsId, projectId, cronJobId);
});
