import { createBffHandler } from "@sparkops/bff-core";
import { purgeFileNode } from "@sparkops/bff-modules/filesystem";

export const DELETE = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const nodeId = params?.nodeId;
  if (!wsId || !nodeId) {
    throw new Error("workspace and node id required");
  }
  const idempotencyKey = req.headers.get("idempotency-key")?.trim();
  if (!idempotencyKey) {
    throw new Error("Idempotency-Key header required");
  }
  const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
  await purgeFileNode(ctx, wsId, nodeId, idempotencyKey, namespaceId);
  return { purged: true };
});