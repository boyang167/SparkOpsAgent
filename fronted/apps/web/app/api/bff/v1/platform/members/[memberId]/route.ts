import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deletePlatformMember, updatePlatformMember } from "@sparkops/bff-modules/identity";
import { UpdatePlatformMemberCommandSchema } from "@sparkops/contracts/identity";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ memberId: string }> };

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { memberId } = await params;
    const body = UpdatePlatformMemberCommandSchema.parse(await req.json());
    await updatePlatformMember(ctx, memberId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const DELETE = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { memberId } = await params;
    await deletePlatformMember(ctx, memberId);
    return ok({ deleted: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
