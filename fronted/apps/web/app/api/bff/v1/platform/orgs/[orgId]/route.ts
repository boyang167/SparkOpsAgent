import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deletePlatformOrganization } from "@sparkops/bff-modules/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const DELETE = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    await deletePlatformOrganization(ctx, orgId);
    return ok({ deleted: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
