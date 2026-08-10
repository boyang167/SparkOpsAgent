import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listBuilds, triggerBuild } from "@sparkops/bff-modules/delivery";
import { TriggerBuildCommandSchema } from "@sparkops/contracts/delivery";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const projectId = params?.projectId;
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!projectId || !wsId) {
    throw new Error("project id and workspace required");
  }
  const items = await listBuilds(ctx, wsId, projectId);
  return { items };
});

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; projectId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = TriggerBuildCommandSchema.parse(await req.json().catch(() => ({})));
    const build = await triggerBuild(ctx, params.wsId, params.projectId, body);
    return ok(build, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
