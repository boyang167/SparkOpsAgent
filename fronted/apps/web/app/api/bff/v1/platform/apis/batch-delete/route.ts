import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { DeleteApisCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export const POST = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const body = DeleteApisCommandSchema.parse(await req.json());
    const ctx = await createBffContext(req);
    const result = await ctx.upstream.governance.deletePlatformApisByIds(body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
