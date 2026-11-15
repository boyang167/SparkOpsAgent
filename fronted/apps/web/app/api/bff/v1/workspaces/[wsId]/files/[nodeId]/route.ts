import { createBffHandler } from "@sparkops/bff-core";
import { getFileNode } from "@sparkops/bff-modules/filesystem";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const nodeId = params?.nodeId;
  if (!wsId || !nodeId) {
    throw new Error("workspace and node id required");
  }
  const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
  return getFileNode(ctx, wsId, nodeId, namespaceId);
});
