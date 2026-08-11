import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listAppLogs } from "@sparkops/bff-modules/platform";
import { ListAppLogsQuerySchema } from "@sparkops/contracts/platform";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const url = new URL(req.url);
    const query = ListAppLogsQuerySchema.parse({
      page: url.searchParams.get("page") ? Number(url.searchParams.get("page")) : undefined,
      page_size: url.searchParams.get("page_size")
        ? Number(url.searchParams.get("page_size"))
        : undefined,
      level: url.searchParams.get("level") || undefined,
      category: url.searchParams.get("category") || undefined,
      trace_id: url.searchParams.get("trace_id") || undefined,
      keyword: url.searchParams.get("keyword") || undefined,
    });
    const data = await listAppLogs(ctx, params.wsId, query);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
