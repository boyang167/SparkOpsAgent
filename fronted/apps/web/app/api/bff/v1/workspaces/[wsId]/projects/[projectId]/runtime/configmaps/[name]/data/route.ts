import { createBffHandler } from "@sparkops/bff-core";
import { deleteConfigMapEntry, upsertConfigMapEntry } from "@sparkops/bff-modules/cluster";

export const PUT = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const name = params?.name;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId || !name) {
    throw new Error("project id, workspace and name required");
  }
  const body = (await req.json()) as Record<string, unknown>;
  return upsertConfigMapEntry(ctx, wsId, projectId, name, body);
});

export const DELETE = createBffHandler(async (ctx, req, { params }) => {
  const projectId = params?.projectId;
  const name = params?.name;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId || !name) {
    throw new Error("project id, workspace and name required");
  }
  const key = new URL(req.url).searchParams.get("key")?.trim() ?? "";
  if (!key) {
    throw new Error("key is required");
  }
  return deleteConfigMapEntry(ctx, wsId, projectId, name, key);
});
