import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getMyOrgPermissions } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const permissions = await getMyOrgPermissions(ctx, orgId);
    return ok({ permissions }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
