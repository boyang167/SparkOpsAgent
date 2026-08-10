import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { updatePlatformOrganizationStatus } from "@sparkops/bff-modules/tenant";
import { UpdateOrgStatusCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const body = UpdateOrgStatusCommandSchema.parse(await req.json());
    await updatePlatformOrganizationStatus(ctx, orgId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
