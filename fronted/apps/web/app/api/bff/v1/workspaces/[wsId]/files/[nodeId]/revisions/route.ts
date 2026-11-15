import { createBffHandler } from "@sparkops/bff-core";
import { listFileRevisions } from "@sparkops/bff-modules/filesystem";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const nodeId = params?.nodeId;
  if (!wsId || !nodeId) {
    throw new Error("workspace and node id required");
  }
  const cursor = req.nextUrl.searchParams.get("cursor") ?? undefined;
  const limitRaw = req.nextUrl.searchParams.get("limit");
  const limit = limitRaw ? Number.parseInt(limitRaw, 10) : undefined;
  const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
  return listFileRevisions(ctx, wsId, nodeId, { cursor, limit, namespace_id: namespaceId });
});
