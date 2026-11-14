import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { approveShareRequest } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ requestId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    if (!idempotencyKey) {
      throw new BffError("Idempotency-Key header required", "FILESYSTEM_IDEMPOTENCY_KEY_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const { requestId } = await context.params;
    return ok(
      await approveShareRequest(ctx, organizationId, ctx.workspaceId, requestId, idempotencyKey),
      traceId,
    );
  } catch (error) {
    return fail(error, traceId);
  }
}
