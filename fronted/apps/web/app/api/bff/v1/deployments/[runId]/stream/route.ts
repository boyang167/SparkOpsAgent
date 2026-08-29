import { createBffContext, createTraceId, fail, proxyUpstreamStream } from "@sparkops/bff-core";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest, context: { params: Promise<{ runId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { runId } = await context.params;
    return proxyUpstreamStream(ctx, `/api/v1/deployments/${runId}/stream`);
  } catch (err) {
    return fail(err, traceId);
  }
}
