import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { markAllNotificationsRead } from "@sparkops/bff-modules/notification";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const result = await markAllNotificationsRead(ctx, ctx.workspaceId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
