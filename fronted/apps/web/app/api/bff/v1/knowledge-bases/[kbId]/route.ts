import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { archiveKnowledgeBase, getKnowledgeBase, updateKnowledgeBase } from "@sparkops/bff-modules/knowledge";
import { UpdateKnowledgeBaseCommandSchema } from "@sparkops/contracts/knowledge";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const kbId = params?.kbId;
  const wsId = ctx.workspaceId;
  if (!kbId || !wsId) {
    throw new Error("knowledge base id and workspace required");
  }
  return getKnowledgeBase(ctx, wsId, kbId);
});

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ kbId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = UpdateKnowledgeBaseCommandSchema.parse(await req.json());
    const kb = await updateKnowledgeBase(ctx, ctx.workspaceId, params.kbId, body);
    return ok(kb, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ kbId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const result = await archiveKnowledgeBase(ctx, ctx.workspaceId, params.kbId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
