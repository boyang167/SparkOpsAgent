import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { batchDeleteOperationRecords } from "@sparkops/bff-modules/platform";
import { BatchDeleteOperationRecordsCommandSchema } from "@sparkops/contracts/platform";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = BatchDeleteOperationRecordsCommandSchema.parse(await req.json());
    const result = await batchDeleteOperationRecords(ctx, params.wsId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
