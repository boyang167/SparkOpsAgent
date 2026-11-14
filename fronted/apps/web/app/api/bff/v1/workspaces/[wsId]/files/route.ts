import { createBffHandler } from "@sparkops/bff-core";
import { listFileNodes } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const parentId = req.nextUrl.searchParams.get("parent_id") ?? undefined;
  const typeValue = req.nextUrl.searchParams.get("type") ?? undefined;
  if (typeValue && typeValue !== "FILE" && typeValue !== "DIRECTORY") {
    throw new BffError("unsupported filesystem node type", "INVALID_REQUEST", 400);
  }
  const nodeType =
    typeValue === "FILE" || typeValue === "DIRECTORY" ? typeValue : undefined;
  const query = req.nextUrl.searchParams.get("q") ?? undefined;
  const cursor = req.nextUrl.searchParams.get("cursor") ?? undefined;
  const limitRaw = req.nextUrl.searchParams.get("limit");
  const limit = limitRaw ? Number.parseInt(limitRaw, 10) : undefined;
  return listFileNodes(ctx, wsId, {
    parent_id: parentId,
    type: nodeType,
    q: query,
    cursor,
    limit,
  });
});
