import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getOrganization } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const org = await getOrganization(ctx, orgId);
    return ok(org, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
