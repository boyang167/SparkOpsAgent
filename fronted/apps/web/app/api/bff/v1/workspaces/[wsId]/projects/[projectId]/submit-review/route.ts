import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { submitProjectReview } from "@sparkops/bff-modules/workspace";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; projectId: string }> };

export async function POST(req: NextRequest, { params }: RouteParams) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId } = await params;
    return ok(await submitProjectReview(ctx, wsId, projectId), traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
