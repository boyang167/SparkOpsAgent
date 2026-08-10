import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import {
  addProjectMember,
  listProjectMembers,
  removeProjectMember,
  updateProjectMemberRole,
} from "@sparkops/bff-modules/workspace";
import {
  AddProjectMemberCommandSchema,
  UpdateProjectMemberRoleCommandSchema,
} from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; projectId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId } = await params;
    const items = await listProjectMembers(ctx, wsId, projectId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId } = await params;
    const body = AddProjectMemberCommandSchema.parse(await req.json());
    await addProjectMember(ctx, wsId, projectId, body);
    return ok({ added: true }, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
};
