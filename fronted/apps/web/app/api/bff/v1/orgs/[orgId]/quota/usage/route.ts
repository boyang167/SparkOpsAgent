import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getOrgQuotaUsage } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const usage = await getOrgQuotaUsage(ctx, orgId);
    return ok(usage, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
