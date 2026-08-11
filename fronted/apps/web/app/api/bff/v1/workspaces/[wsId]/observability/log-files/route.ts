import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listLogFiles } from "@sparkops/bff-modules/platform";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const data = await listLogFiles(ctx, params.wsId);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
