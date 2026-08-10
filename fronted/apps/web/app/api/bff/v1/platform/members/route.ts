import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createPlatformMember, listPlatformMembers } from "@sparkops/bff-modules/identity";
import { CreatePlatformMemberCommandSchema } from "@sparkops/contracts/identity";
import { NextRequest } from "next/server";

export const GET = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const items = await listPlatformMembers(ctx);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = CreatePlatformMemberCommandSchema.parse(await req.json());
    const member = await createPlatformMember(ctx, body);
    return ok(member, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
};
