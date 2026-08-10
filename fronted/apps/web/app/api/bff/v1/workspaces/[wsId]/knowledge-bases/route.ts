import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createKnowledgeBase, listKnowledgeBases } from "@sparkops/bff-modules/knowledge";
import { CreateKnowledgeBaseCommandSchema } from "@sparkops/contracts/knowledge";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const items = await listKnowledgeBases(ctx, wsId);
  return { items };
});

export async function POST(
  req: NextRequest,
  context: { params: Promise<{ wsId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateKnowledgeBaseCommandSchema.parse(await req.json());
    const kb = await createKnowledgeBase(ctx, params.wsId, body);
    return ok(kb, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
