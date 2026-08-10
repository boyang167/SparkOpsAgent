import { createBffContext, createTraceId, createBffHandler, fail, ok } from "@sparkops/bff-core";
import { getAgent, updateAgent, archiveAgent } from "@sparkops/bff-modules/agent";
import { UpdateAgentCommandSchema } from "@sparkops/contracts/agent";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const agentId = params?.agentId;
  const wsId = ctx.workspaceId;
  if (!agentId || !wsId) {
    throw new Error("agent id and workspace required");
  }
  return getAgent(ctx, wsId, agentId);
});

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ agentId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = UpdateAgentCommandSchema.parse(await req.json());
    const agent = await updateAgent(ctx, ctx.workspaceId, params.agentId, body);
    return ok(agent, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ agentId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const result = await archiveAgent(ctx, ctx.workspaceId, params.agentId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
