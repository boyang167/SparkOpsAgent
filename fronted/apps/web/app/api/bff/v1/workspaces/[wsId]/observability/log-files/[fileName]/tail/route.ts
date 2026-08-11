import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { tailLogFile } from "@sparkops/bff-modules/platform";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; fileName: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const url = new URL(req.url);
    const offset = url.searchParams.get("offset") ? Number(url.searchParams.get("offset")) : 0;
    const limit = url.searchParams.get("limit") ? Number(url.searchParams.get("limit")) : 200;
    const data = await tailLogFile(ctx, params.wsId, decodeURIComponent(params.fileName), offset, limit);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
