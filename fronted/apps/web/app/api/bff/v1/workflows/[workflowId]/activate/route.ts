import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { activateWorkflow } from "@sparkops/bff-modules/workflow";
import { NextRequest } from "next/server";

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
    const workflow = await activateWorkflow(ctx, ctx.workspaceId, params.workflowId);
    return ok(workflow, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
