import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { testSecretConnection } from "@sparkops/bff-modules/secret";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; secretId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await testSecretConnection(ctx, params.wsId, params.secretId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
