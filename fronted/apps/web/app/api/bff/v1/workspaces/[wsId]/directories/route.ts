import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createDirectoryNode } from "@sparkops/bff-modules/filesystem";
import { CreateDirectoryCommandSchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateDirectoryCommandSchema.parse(await req.json());
    const idempotencyKey = req.headers.get("idempotency-key")?.trim();
    if (!idempotencyKey) {
      throw new Error("Idempotency-Key header required");
    }
    const node = await createDirectoryNode(ctx, params.wsId, body, idempotencyKey);
    return ok(node, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
