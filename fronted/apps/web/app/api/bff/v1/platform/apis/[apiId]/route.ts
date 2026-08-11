import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { UpdateAPIMetaCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ apiId: string }> };

export const PATCH = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { apiId } = await context.params;
    const body = UpdateAPIMetaCommandSchema.parse(await req.json());
    const ctx = await createBffContext(req);
    const result = await ctx.upstream.governance.updatePlatformApiMeta(apiId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const DELETE = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { apiId } = await context.params;
    const ctx = await createBffContext(req);
    const result = await ctx.upstream.governance.deletePlatformApi(apiId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
