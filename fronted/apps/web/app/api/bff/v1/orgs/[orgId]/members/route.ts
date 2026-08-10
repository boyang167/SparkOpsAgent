import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getMyOrgPermissions, inviteOrgMember, listOrgMembers, removeOrgMember, updateOrgMemberRole } from "@sparkops/bff-modules/tenant";
import { InviteOrgMemberCommandSchema, UpdateOrgMemberRoleCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const items = await listOrgMembers(ctx, orgId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const body = InviteOrgMemberCommandSchema.parse(await req.json());
    const member = await inviteOrgMember(ctx, orgId, body);
    return ok(member, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
};
