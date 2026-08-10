import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { startOIDCLogin } from "@sparkops/bff-modules/identity";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const data = await startOIDCLogin(ctx);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
