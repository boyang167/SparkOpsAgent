import { createBffHandler } from "@sparkops/bff-core";
import { createConfigMap, listConfigMaps } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  return listConfigMaps(ctx, wsId, projectId);
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  const body = (await req.json().catch(() => ({}))) as Record<string, unknown>;
  return createConfigMap(ctx, wsId, projectId, body);
});
