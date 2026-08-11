import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getRoleMenus, setRoleMenus } from "@sparkops/bff-modules/governance";
import { SetRoleMenusCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; roleId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const menuIds = await getRoleMenus(ctx, params.wsId, params.roleId);
    return ok({ menu_ids: menuIds }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function PUT(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; roleId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = SetRoleMenusCommandSchema.parse(await req.json());
    const result = await setRoleMenus(ctx, params.wsId, params.roleId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
