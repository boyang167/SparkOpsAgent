import { createBffContext, createTraceId, fail, proxyUpstreamStream } from "@sparkops/bff-core";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ clusterId: string; namespace: string; pod: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const path =
      `/api/v1/plugins/kubernetes/clusters/${encodeURIComponent(params.clusterId)}` +
      `/namespaces/${encodeURIComponent(params.namespace)}` +
      `/pods/${encodeURIComponent(params.pod)}/logs${req.nextUrl.search}`;
    return proxyUpstreamStream(ctx, path);
  } catch (err) {
    return fail(err, traceId);
  }
}
