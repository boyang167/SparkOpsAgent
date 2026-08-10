import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { archiveWorkflow, getWorkflow, updateWorkflow } from "@sparkops/bff-modules/workflow";
import { UpdateWorkflowCommandSchema } from "@sparkops/contracts/workflow";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const workflowId = params?.workflowId;
  const wsId = ctx.workspaceId;
  if (!workflowId || !wsId) {
    throw new Error("workflow id and workspace required");
  }
  return getWorkflow(ctx, wsId, workflowId);
});

export async function PATCH(
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
    const body = UpdateWorkflowCommandSchema.parse(await req.json());
    const workflow = await updateWorkflow(ctx, ctx.workspaceId, params.workflowId, body);
    return ok(workflow, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
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
    const result = await archiveWorkflow(ctx, ctx.workspaceId, params.workflowId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
