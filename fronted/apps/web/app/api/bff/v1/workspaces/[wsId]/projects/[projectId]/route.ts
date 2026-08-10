import { createBffContext, createTraceId, createBffHandler, fail, ok } from "@sparkops/bff-core";
import { getProject, updateProject, archiveProject } from "@sparkops/bff-modules/workspace";
import { UpdateProjectCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) {
    throw new Error("workspace id and project id required");
  }
  return getProject(ctx, wsId, projectId);
});

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; projectId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = UpdateProjectCommandSchema.parse(await req.json());
    const project = await updateProject(ctx, params.wsId, params.projectId, body);
    return ok(project, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; projectId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await archiveProject(ctx, params.wsId, params.projectId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
