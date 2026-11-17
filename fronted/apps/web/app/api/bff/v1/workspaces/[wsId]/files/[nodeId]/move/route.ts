import { createBffHandler } from "@sparkops/bff-core";
import { moveFileNode } from "@sparkops/bff-modules/filesystem";

export const POST = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const nodeId = params?.nodeId;
  if (!wsId || !nodeId) {
    throw new Error("workspace and node id required");
  }
  const idempotencyKey = req.headers.get("idempotency-key")?.trim();
  if (!idempotencyKey) {
    throw new Error("Idempotency-Key header required");
  }
  const body = (await req.json()) as { parent_id?: unknown; name?: unknown };
  if (
    (body.parent_id !== null && typeof body.parent_id !== "string") ||
    typeof body.name !== "string" ||
    !body.name.trim()
  ) {
    throw new Error("invalid move request");
  }
  const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
  await moveFileNode(
    ctx,
    wsId,
    nodeId,
    { parent_id: body.parent_id, name: body.name },
    idempotencyKey,
    namespaceId,
  );
  return { moved: true };
});
