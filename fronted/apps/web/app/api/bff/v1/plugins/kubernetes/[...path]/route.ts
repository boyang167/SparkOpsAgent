import { createBffContext, createTraceId, fail } from "@sparkops/bff-core";
import { proxyKubernetesPluginRequest } from "@sparkops/bff-modules/plugins";
import { NextRequest } from "next/server";

type KubernetesRouteContext = {
  params: Promise<{ path: string[] }>;
};

async function handle(req: NextRequest, context: KubernetesRouteContext) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    return proxyKubernetesPluginRequest(ctx, req, params.path, req.nextUrl.search);
  } catch (err) {
    return fail(err, traceId);
  }
}

export const GET = handle;
export const HEAD = handle;
export const POST = handle;
export const PUT = handle;
export const PATCH = handle;
export const DELETE = handle;
