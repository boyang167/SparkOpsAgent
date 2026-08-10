import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { assignRole } from "@sparkops/bff-modules/governance";
import { AssignRoleCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; memberId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = AssignRoleCommandSchema.parse(await req.json());
    const result = await assignRole(ctx, params.wsId, params.memberId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
