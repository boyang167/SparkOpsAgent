import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteStorageProvider, updateStorageProvider } from "@sparkops/bff-modules/filesystem";
import { UpdateStorageProviderCommandSchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; providerId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = UpdateStorageProviderCommandSchema.parse(await req.json());
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const updated = await updateStorageProvider(ctx, params.wsId, params.providerId, body, idempotencyKey);
    return ok(updated, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; providerId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const deleted = await deleteStorageProvider(ctx, params.wsId, params.providerId, idempotencyKey);
    return ok(deleted, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}