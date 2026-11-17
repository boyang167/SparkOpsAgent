import { NextResponse } from "next/server";
import { createBffRawHandler } from "@sparkops/bff-core";
import { loadFilePreviewContent } from "@sparkops/bff-modules/filesystem";

export const runtime = "nodejs";

export const GET = createBffRawHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  const nodeId = params?.nodeId;
  if (!wsId || !nodeId) {
    throw new Error("workspace and node id required");
  }
  const search = req.nextUrl.searchParams;
  const loaded = await loadFilePreviewContent(ctx, wsId, nodeId, {
    revision_id: search.get("revision_id") ?? undefined,
    namespace_id: search.get("namespace_id") ?? undefined,
  });
  return new NextResponse(Buffer.from(loaded.body), {
    status: 200,
    headers: {
      "Content-Type": loaded.mimeType,
      "X-Preview-Kind": loaded.previewKind,
      "Cache-Control": "private, no-store",
      "X-Trace-Id": ctx.traceId,
    },
  });
});