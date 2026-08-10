import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { inviteMember, listMembers } from "@sparkops/bff-modules/governance";
import { InviteMemberCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const items = await listMembers(ctx, wsId);
  return { items };
});

export async function POST(req: NextRequest, context: { params: Promise<{ wsId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = InviteMemberCommandSchema.parse(await req.json());
    const member = await inviteMember(ctx, params.wsId, body);
    return ok(member, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
