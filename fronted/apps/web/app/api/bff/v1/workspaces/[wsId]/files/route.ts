import { createBffHandler } from "@sparkops/bff-core";
import { listFileNodes } from "@sparkops/bff-modules/filesystem";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const parentId = req.nextUrl.searchParams.get("parent_id") ?? undefined;
  const cursor = req.nextUrl.searchParams.get("cursor") ?? undefined;
  const limitRaw = req.nextUrl.searchParams.get("limit");
  const limit = limitRaw ? Number.parseInt(limitRaw, 10) : undefined;
  return listFileNodes(ctx, wsId, { parent_id: parentId, cursor, limit });
});
