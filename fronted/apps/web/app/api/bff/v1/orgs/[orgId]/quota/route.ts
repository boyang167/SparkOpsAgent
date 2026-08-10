import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getOrgQuota, getOrgQuotaUsage, updateOrgQuota } from "@sparkops/bff-modules/tenant";
import { UpdateOrgQuotaCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const quota = await getOrgQuota(ctx, orgId);
    return ok(quota, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const body = UpdateOrgQuotaCommandSchema.parse(await req.json());
    const quota = await updateOrgQuota(ctx, orgId, body);
    return ok(quota, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
