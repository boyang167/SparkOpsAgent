import { createBffContext, createBffHandler, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createDocument, listDocuments } from "@sparkops/bff-modules/knowledge";
import { CreateDocumentCommandSchema } from "@sparkops/contracts/knowledge";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const kbId = params?.kbId;
  const wsId = ctx.workspaceId;
  if (!kbId || !wsId) {
    throw new Error("knowledge base id and workspace required");
  }
  const items = await listDocuments(ctx, wsId, kbId);
  return { items };
});

export async function POST(
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
    const body = CreateDocumentCommandSchema.parse(await req.json());
    const doc = await createDocument(ctx, ctx.workspaceId, params.kbId, body);
    return ok(doc, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
