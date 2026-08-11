import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteOperationRecord } from "@sparkops/bff-modules/platform";
import { NextRequest } from "next/server";

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; recordId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await deleteOperationRecord(ctx, params.wsId, params.recordId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
