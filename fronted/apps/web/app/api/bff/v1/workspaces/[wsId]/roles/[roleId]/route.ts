import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteRole, updateRole } from "@sparkops/bff-modules/governance";
import { UpdateRoleCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; roleId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = UpdateRoleCommandSchema.parse(await req.json());
    const role = await updateRole(ctx, params.wsId, params.roleId, body);
    return ok(role, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; roleId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await deleteRole(ctx, params.wsId, params.roleId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
