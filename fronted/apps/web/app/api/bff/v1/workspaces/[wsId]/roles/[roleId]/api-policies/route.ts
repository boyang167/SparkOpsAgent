import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listRoleAPIPolicies, updateRoleAPIPolicies } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; roleId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, roleId } = await params;
    const items = await listRoleAPIPolicies(ctx, wsId, roleId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const PUT = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, roleId } = await params;
    const body = (await req.json()) as {
      items: import("@sparkops/contracts/governance").RoleAPIPolicy[];
    };
    const result = await updateRoleAPIPolicies(ctx, wsId, roleId, body.items ?? []);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
