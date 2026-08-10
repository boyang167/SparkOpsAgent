import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { changePassword } from "@sparkops/bff-modules/identity";
import { ChangePasswordCommandSchema } from "@sparkops/contracts/identity";
import { NextRequest } from "next/server";

export async function PATCH(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = ChangePasswordCommandSchema.parse(await req.json());
    const result = await changePassword(ctx, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
