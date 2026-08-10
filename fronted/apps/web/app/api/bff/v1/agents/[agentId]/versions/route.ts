import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { addAgentVersion } from "@sparkops/bff-modules/agent";
import { AddAgentVersionCommandSchema } from "@sparkops/contracts/agent";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest, context: { params: Promise<{ agentId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = AddAgentVersionCommandSchema.parse(await req.json());
    const result = await addAgentVersion(ctx, ctx.workspaceId, params.agentId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
