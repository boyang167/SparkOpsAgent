import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listMyMenuButtons } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId } = await params;
    const path = req.nextUrl.searchParams.get("path");
    if (!path) {
      return fail(new Error("path query required"), traceId);
    }
    const data = await listMyMenuButtons(ctx, wsId, path);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
