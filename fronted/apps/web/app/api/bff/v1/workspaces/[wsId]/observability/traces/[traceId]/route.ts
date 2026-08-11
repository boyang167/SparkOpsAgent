import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getTraceDetail } from "@sparkops/bff-modules/platform";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; traceId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const data = await getTraceDetail(ctx, params.wsId, decodeURIComponent(params.traceId));
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
