import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { copyRole } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; roleId: string }> };

export const POST = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, roleId } = await params;
    const body = (await req.json().catch(() => ({}))) as { name?: string };
    const role = await copyRole(ctx, wsId, roleId, body.name);
    return ok(role, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
