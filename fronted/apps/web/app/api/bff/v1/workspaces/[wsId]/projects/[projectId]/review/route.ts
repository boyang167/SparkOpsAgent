import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { reviewProject } from "@sparkops/bff-modules/workspace";
import { DecideProjectReviewCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; projectId: string }> };

export async function POST(req: NextRequest, { params }: RouteParams) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId } = await params;
    const input = DecideProjectReviewCommandSchema.parse(await req.json());
    return ok(await reviewProject(ctx, wsId, projectId, input), traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
