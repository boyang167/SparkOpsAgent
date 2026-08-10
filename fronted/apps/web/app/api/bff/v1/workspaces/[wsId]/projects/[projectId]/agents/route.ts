import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createAgent, listAgents } from "@sparkops/bff-modules/agent";
import { CreateAgentCommandSchema } from "@sparkops/contracts/agent";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  const projectId = params?.projectId;
  if (!wsId || !projectId) {
    throw new Error("workspace id and project id required");
  }
  const items = await listAgents(ctx, wsId, projectId);
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
    const body = CreateAgentCommandSchema.parse(await req.json());
    const agent = await createAgent(ctx, params.wsId, params.projectId, body);
    return ok(agent, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
