import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { confirmApiSync, previewApiSync } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ wsId: string }> };

export const GET = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { wsId } = await context.params;
    const ctx = await createBffContext(req);
    const diff = await previewApiSync(ctx, wsId);
    return ok(diff, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { wsId } = await context.params;
    const ctx = await createBffContext(req);
    const body = (await req.json()) as { new: unknown[]; delete: unknown[] };
    const result = await confirmApiSync(ctx, wsId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
