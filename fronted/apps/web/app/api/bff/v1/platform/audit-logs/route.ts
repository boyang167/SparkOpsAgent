import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listPlatformAuditLogs } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

export const GET = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const items = await listPlatformAuditLogs(ctx);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
