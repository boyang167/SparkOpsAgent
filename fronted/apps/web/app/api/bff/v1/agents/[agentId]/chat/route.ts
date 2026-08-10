import { createBffContext, createTraceId, fail } from "@sparkops/bff-core";
import { StartChatCommandSchema } from "@sparkops/contracts/runtime";
import { startChat } from "@sparkops/bff-modules/runtime";
import { NextRequest } from "next/server";

export async function POST(req: NextRequest, context: { params: Promise<{ agentId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = StartChatCommandSchema.parse(await req.json());
    const result = await startChat(ctx, params.agentId, body);
    return Response.json(
      { code: 0, message: "ok", data: result, trace_id: traceId },
      { status: 200 },
    );
  } catch (err) {
    return fail(err, traceId);
  }
}
