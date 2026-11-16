import { createBffHandler } from "@sparkops/bff-core";
import { getFileStorage } from "@sparkops/bff-modules/filesystem";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const nodeId = params?.nodeId;
  if (!wsId || !nodeId) {
    throw new Error("workspace and node id required");
  }
  const search = req.nextUrl.searchParams;
  return getFileStorage(ctx, wsId, nodeId, {
    revision_id: search.get("revision_id") ?? undefined,
    namespace_id: search.get("namespace_id") ?? undefined,
  });
});