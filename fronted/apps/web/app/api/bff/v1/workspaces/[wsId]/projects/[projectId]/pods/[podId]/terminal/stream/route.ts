import { createBffContext, createTraceId, fail, proxyUpstreamStream } from "@sparkops/bff-core";
import { NextRequest } from "next/server";

export async function GET(
  _req: NextRequest,
  context: { params: Promise<{ wsId: string; projectId: string; podId: string }> },
) {
  const traceId = createTraceId(_req);
  try {
    const ctx = await createBffContext(_req);
    const params = await context.params;
    return proxyUpstreamStream(
      ctx,
      `/api/v1/projects/${params.projectId}/pods/${params.podId}/terminal/stream`,
    );
  } catch (err) {
    return fail(err, traceId);
  }
}
