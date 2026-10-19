import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getMetricsSummary } from "@sparkops/bff-modules/platform";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const page = Number(req.nextUrl.searchParams.get("page") ?? "1");
    const pageSize = Number(req.nextUrl.searchParams.get("page_size") ?? "20");
    const data = await getMetricsSummary(ctx, params.wsId, {
      page: Number.isFinite(page) && page > 0 ? page : 1,
      page_size: Number.isFinite(pageSize) && pageSize > 0 ? pageSize : 20,
    });
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
