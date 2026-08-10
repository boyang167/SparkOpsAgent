import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { removeRoleBinding } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; memberId: string; roleId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await removeRoleBinding(ctx, params.wsId, params.memberId, params.roleId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
