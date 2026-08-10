import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listPlatformApis } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

export const GET = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const items = await listPlatformApis(ctx);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
