import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { abortTransferSession, getTransferSession } from "@sparkops/bff-modules/filesystem";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  const sessionId = params?.sessionId;
  if (!wsId || !sessionId) {
    throw new Error("workspace and session id required");
  }
  return getTransferSession(ctx, wsId, sessionId);
});

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; sessionId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    await abortTransferSession(ctx, params.wsId, params.sessionId, idempotencyKey);
    return ok({ aborted: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
