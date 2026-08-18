import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listGeneratedTemplates } from "@sparkops/bff-modules/workspace";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; projectId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId } = await params;
    const data = await listGeneratedTemplates(ctx, wsId, projectId);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
