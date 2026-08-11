import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listRoleDataPolicies, updateRoleDataPolicies } from "@sparkops/bff-modules/governance";
import { UpdateRoleDataPoliciesCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteContext = { params: Promise<{ wsId: string; roleId: string }> };

export const GET = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { wsId, roleId } = await context.params;
    const ctx = await createBffContext(req);
    const items = await listRoleDataPolicies(ctx, wsId, roleId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const PUT = async (req: NextRequest, context: RouteContext) => {
  const traceId = createTraceId(req);
  try {
    const { wsId, roleId } = await context.params;
    const body = UpdateRoleDataPoliciesCommandSchema.parse(await req.json());
    const ctx = await createBffContext(req);
    const result = await updateRoleDataPolicies(ctx, wsId, roleId, body.items);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
