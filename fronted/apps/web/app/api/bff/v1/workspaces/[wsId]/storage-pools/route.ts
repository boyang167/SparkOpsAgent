import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createStoragePool, listStoragePools } from "@sparkops/bff-modules/filesystem";
import { CreateStoragePoolCommandSchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  return listStoragePools(ctx, wsId);
});

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateStoragePoolCommandSchema.parse(await req.json());
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const created = await createStoragePool(ctx, params.wsId, body, idempotencyKey);
    return ok(created, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
