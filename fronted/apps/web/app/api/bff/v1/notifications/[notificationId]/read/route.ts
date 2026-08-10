import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { markNotificationRead } from "@sparkops/bff-modules/notification";
import { NextRequest } from "next/server";

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ notificationId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const result = await markNotificationRead(ctx, ctx.workspaceId, params.notificationId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
