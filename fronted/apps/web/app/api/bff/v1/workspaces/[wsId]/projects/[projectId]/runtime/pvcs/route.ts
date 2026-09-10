import { createBffHandler } from "@sparkops/bff-core";
import { createPVC, listPVCs } from "@sparkops/bff-modules/cluster";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  return listPVCs(ctx, wsId, projectId);
});

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  const body = (await req.json().catch(() => ({}))) as Record<string, unknown>;
  return createPVC(ctx, wsId, projectId, body);
});
