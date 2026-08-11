import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listNavMenus } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId } = await params;
    const items = await listNavMenus(ctx, wsId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
