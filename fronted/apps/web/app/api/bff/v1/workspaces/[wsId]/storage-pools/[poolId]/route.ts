import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteStoragePool, updateStoragePool } from "@sparkops/bff-modules/filesystem";
import { UpdateStoragePoolCommandSchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; poolId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = UpdateStoragePoolCommandSchema.parse(await req.json());
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const updated = await updateStoragePool(ctx, params.wsId, params.poolId, body, idempotencyKey);
    return ok(updated, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; poolId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const deleted = await deleteStoragePool(ctx, params.wsId, params.poolId, idempotencyKey);
    return ok(deleted, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}