import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { publishAgent } from "@sparkops/bff-modules/agent";
import { PublishAgentCommandSchema } from "@sparkops/contracts/agent";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest, context: { params: Promise<{ agentId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = PublishAgentCommandSchema.parse(await req.json());
    const result = await publishAgent(ctx, ctx.workspaceId, params.agentId, body.version_id);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
