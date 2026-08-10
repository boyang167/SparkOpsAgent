import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { searchDocuments } from "@sparkops/bff-modules/knowledge";
import { SearchDocumentsCommandSchema } from "@sparkops/contracts/knowledge";
import { NextRequest } from "next/server";

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
    const body = SearchDocumentsCommandSchema.parse(await req.json());
    const items = await searchDocuments(ctx, ctx.workspaceId, params.kbId, body);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
