import { createBffHandler } from "@sparkops/bff-core";
import { emptyTrash, listTrashItems } from "@sparkops/bff-modules/filesystem";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
  const cursor = req.nextUrl.searchParams.get("cursor") ?? undefined;
  const limitRaw = req.nextUrl.searchParams.get("limit");
  const limit = limitRaw ? Number.parseInt(limitRaw, 10) : undefined;
  return listTrashItems(ctx, wsId, {
    namespace_id: namespaceId,
    cursor,
    limit,
  });
});

export const DELETE = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const idempotencyKey = req.headers.get("idempotency-key")?.trim();
  if (!idempotencyKey) {
    throw new Error("Idempotency-Key header required");
  }
  const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
  return emptyTrash(ctx, wsId, idempotencyKey, namespaceId);
});