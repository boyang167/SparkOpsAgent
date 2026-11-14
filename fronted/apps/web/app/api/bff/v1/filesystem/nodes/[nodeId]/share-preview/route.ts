import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { previewGovernedShare } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { CreateGovernedGrantCommandSchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ nodeId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new BffError(
        "Idempotency-Key header required",
        "FILESYSTEM_IDEMPOTENCY_KEY_REQUIRED",
        400,
        traceId,
      );
    }
    const ctx = await createBffContext(req);
    const { nodeId } = await context.params;
    const body = CreateGovernedGrantCommandSchema.parse(await req.json());
    return ok(
      await previewGovernedShare(
        ctx,
        organizationId,
        ctx.workspaceId,
        nodeId,
        body,
        idempotencyKey,
      ),
      traceId,
    );
  } catch (err) {
    return fail(err, traceId);
  }
}
