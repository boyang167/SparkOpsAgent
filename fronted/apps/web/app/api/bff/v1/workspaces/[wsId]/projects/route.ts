import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createProject, listProjects } from "@sparkops/bff-modules/workspace";
import { CreateProjectCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const items = await listProjects(ctx, wsId);
  return { items };
});

export async function POST(req: NextRequest, context: { params: Promise<{ wsId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateProjectCommandSchema.parse(await req.json());
    const project = await createProject(ctx, params.wsId, body);
    return ok(project, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
