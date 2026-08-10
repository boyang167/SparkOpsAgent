import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { removeProjectMember, updateProjectMemberRole } from "@sparkops/bff-modules/workspace";
import { UpdateProjectMemberRoleCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; projectId: string; memberId: string }> };

export const PATCH = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId, memberId } = await params;
    const body = UpdateProjectMemberRoleCommandSchema.parse(await req.json());
    await updateProjectMemberRole(ctx, wsId, projectId, memberId, body);
    return ok({ updated: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const DELETE = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId, memberId } = await params;
    await removeProjectMember(ctx, wsId, projectId, memberId);
    return ok({ removed: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
