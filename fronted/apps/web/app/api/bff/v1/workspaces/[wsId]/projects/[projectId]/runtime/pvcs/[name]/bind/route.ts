import { createBffHandler } from "@sparkops/bff-core";
import { bindPVC, unbindPVC } from "@sparkops/bff-modules/cluster";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const name = params?.name;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId || !name) {
    throw new Error("project id, workspace and name required");
  }
  const body = (await req.json()) as Record<string, unknown>;
  return bindPVC(ctx, wsId, projectId, name, body);
});

export const DELETE = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const name = params?.name;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId || !name) {
    throw new Error("project id, workspace and name required");
  }
  return unbindPVC(ctx, wsId, projectId, name);
});
