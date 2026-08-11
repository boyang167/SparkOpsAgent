import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getMenuRoles, setMenuRoles } from "@sparkops/bff-modules/governance";
import { SetMenuRolesCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; menuId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, menuId } = await params;
    const role_ids = await getMenuRoles(ctx, wsId, menuId);
    return ok({ role_ids }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const PUT = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, menuId } = await params;
    const body = SetMenuRolesCommandSchema.parse(await req.json());
    await setMenuRoles(ctx, wsId, menuId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
