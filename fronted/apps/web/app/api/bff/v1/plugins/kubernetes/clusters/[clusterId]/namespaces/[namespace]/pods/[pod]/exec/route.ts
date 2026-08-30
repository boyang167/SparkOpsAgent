import { createBffContext, createTraceId, fail } from "@sparkops/bff-core";
import { proxyKubernetesPluginRequest } from "@sparkops/bff-modules/plugins";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ clusterId: string; namespace: string; pod: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    return proxyKubernetesPluginRequest(
      ctx,
      req,
      ["clusters", params.clusterId, "namespaces", params.namespace, "pods", params.pod, "exec"],
      req.nextUrl.search,
    );
  } catch (err) {
    return fail(err, traceId);
  }
}
