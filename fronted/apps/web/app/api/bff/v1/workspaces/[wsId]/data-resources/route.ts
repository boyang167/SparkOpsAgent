import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listDataResources } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ wsId: string }> };

export const GET = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { wsId } = await context.params;
    const ctx = await createBffContext(req);
    const items = await listDataResources(ctx, wsId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
