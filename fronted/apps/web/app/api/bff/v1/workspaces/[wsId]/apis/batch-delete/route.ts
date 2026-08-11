import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteApisByIds } from "@sparkops/bff-modules/governance";
import { DeleteApisCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ wsId: string }> };

export const POST = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { wsId } = await context.params;
    const body = DeleteApisCommandSchema.parse(await req.json());
    const ctx = await createBffContext(req);
    const result = await deleteApisByIds(ctx, wsId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
