import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getAuthProviders } from "@sparkops/bff-modules/identity";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const providers = await getAuthProviders(ctx);
    return ok(providers, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
