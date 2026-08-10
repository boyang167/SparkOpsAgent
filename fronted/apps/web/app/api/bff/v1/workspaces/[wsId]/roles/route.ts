import { createBffHandler, createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createRole, listRoles } from "@sparkops/bff-modules/governance";
import { CreateRoleCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const items = await listRoles(ctx, wsId);
  return { items };
});

export async function POST(req: NextRequest, context: { params: Promise<{ wsId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateRoleCommandSchema.parse(await req.json());
    const role = await createRole(ctx, params.wsId, body);
    return ok(role, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
