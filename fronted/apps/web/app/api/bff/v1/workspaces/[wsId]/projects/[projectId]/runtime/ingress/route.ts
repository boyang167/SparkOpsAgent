import { createBffHandler } from "@sparkops/bff-core";
import { createRuntimeIngress } from "@sparkops/bff-modules/cluster";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  let body: Record<string, unknown> = {};
  try {
    body = (await req.json()) as Record<string, unknown>;
  } catch {
    body = {};
  }
  return createRuntimeIngress(ctx, wsId, projectId, body);
});
