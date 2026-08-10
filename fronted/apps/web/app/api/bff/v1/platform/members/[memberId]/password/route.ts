import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { resetPlatformMemberPassword } from "@sparkops/bff-modules/identity";
import { AdminResetPasswordCommandSchema } from "@sparkops/contracts/identity";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ memberId: string }> };

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { memberId } = await params;
    const body = AdminResetPasswordCommandSchema.parse(await req.json());
    await resetPlatformMemberPassword(ctx, memberId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
