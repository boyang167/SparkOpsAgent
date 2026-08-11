import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listOperationRecords } from "@sparkops/bff-modules/platform";
import { ListOperationRecordsQuerySchema } from "@sparkops/contracts/platform";
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
    const query = ListOperationRecordsQuerySchema.parse({
      page: url.searchParams.get("page") ? Number(url.searchParams.get("page")) : undefined,
      page_size: url.searchParams.get("page_size")
        ? Number(url.searchParams.get("page_size"))
        : undefined,
      ip: url.searchParams.get("ip") || undefined,
      method: url.searchParams.get("method") || undefined,
      path: url.searchParams.get("path") || undefined,
      status: url.searchParams.get("status") ? Number(url.searchParams.get("status")) : undefined,
      member_id: url.searchParams.get("member_id") || undefined,
      trace_id: url.searchParams.get("trace_id") || undefined,
    });
    const data = await listOperationRecords(ctx, params.wsId, query);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
