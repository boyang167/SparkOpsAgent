import { createBffHandler, createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getRoleMenuButtons, setRoleMenuButtons } from "@sparkops/bff-modules/governance";
import { SetRoleMenuButtonsCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  const roleId = params?.roleId;
  if (!wsId || !roleId) {
    throw new Error("workspace id and role id required");
  }
  const button_ids = await getRoleMenuButtons(ctx, wsId, roleId);
  return { button_ids };
});

export async function PUT(req: NextRequest, context: { params: Promise<{ wsId: string; roleId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = SetRoleMenuButtonsCommandSchema.parse(await req.json());
    await setRoleMenuButtons(ctx, params.wsId, params.roleId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
