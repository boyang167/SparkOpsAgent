import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { renameFileNode } from "@sparkops/bff-modules/filesystem";
import { RenameFileNodeCommandSchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; nodeId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = RenameFileNodeCommandSchema.parse(await req.json());
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const namespaceId = req.nextUrl.searchParams.get("namespace_id") ?? undefined;
    await renameFileNode(ctx, params.wsId, params.nodeId, body, idempotencyKey, namespaceId);
    return ok({ renamed: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
