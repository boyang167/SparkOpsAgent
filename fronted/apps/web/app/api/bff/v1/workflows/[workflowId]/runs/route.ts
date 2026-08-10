import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listWorkflowRuns, startWorkflowRun } from "@sparkops/bff-modules/workflow";
import { StartWorkflowRunCommandSchema } from "@sparkops/contracts/workflow";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const workflowId = params?.workflowId;
  const wsId = ctx.workspaceId;
  if (!workflowId || !wsId) {
    throw new Error("workflow id and workspace required");
  }
  const items = await listWorkflowRuns(ctx, wsId, workflowId);
  return { items };
});

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ workflowId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = StartWorkflowRunCommandSchema.parse(await req.json().catch(() => ({})));
    const run = await startWorkflowRun(ctx, ctx.workspaceId, params.workflowId, body.input);
    return ok(run, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
