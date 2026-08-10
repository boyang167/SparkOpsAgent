import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { removeOrgMember, updateOrgMemberRole } from "@sparkops/bff-modules/tenant";
import { UpdateOrgMemberRoleCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string; memberId: string }> };

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId, memberId } = await params;
    const body = UpdateOrgMemberRoleCommandSchema.parse(await req.json());
    await updateOrgMemberRole(ctx, orgId, memberId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const DELETE = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId, memberId } = await params;
    await removeOrgMember(ctx, orgId, memberId);
    return ok({ removed: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
