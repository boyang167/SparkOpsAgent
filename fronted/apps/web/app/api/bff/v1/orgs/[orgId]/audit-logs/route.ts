import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listOrgAuditLogs } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const items = await listOrgAuditLogs(ctx, orgId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
