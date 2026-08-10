import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getSession, updateProfile } from "@sparkops/bff-modules/identity";
import { UpdateProfileCommandSchema } from "@sparkops/contracts/identity";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx) => getSession(ctx));

export async function PATCH(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = UpdateProfileCommandSchema.parse(await req.json());
    const me = await updateProfile(ctx, body);
    return ok(me, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
