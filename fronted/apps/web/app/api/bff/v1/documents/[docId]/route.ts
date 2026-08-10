import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { archiveDocument, updateDocument } from "@sparkops/bff-modules/knowledge";
import { UpdateDocumentCommandSchema } from "@sparkops/contracts/knowledge";
import { NextRequest } from "next/server";

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ docId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const body = UpdateDocumentCommandSchema.parse(await req.json());
    const doc = await updateDocument(ctx, ctx.workspaceId, params.docId, body);
    return ok(doc, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ docId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    if (!ctx.workspaceId) {
      throw new Error("workspace required");
    }
    const result = await archiveDocument(ctx, ctx.workspaceId, params.docId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
