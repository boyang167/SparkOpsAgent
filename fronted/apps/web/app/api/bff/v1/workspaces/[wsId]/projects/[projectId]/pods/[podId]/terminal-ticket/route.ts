import { createBffHandler } from "@sparkops/bff-core";
import { createPodTerminalTicket } from "@sparkops/bff-modules/cluster";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const podId = params?.podId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !podId || !wsId) {
    throw new Error("project id, pod id and workspace required");
  }
  const body = (await req.json()) as Record<string, unknown>;
  return createPodTerminalTicket(ctx, wsId, projectId, podId, body);
});
