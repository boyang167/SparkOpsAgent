import { createBffHandler, createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createTemplate, listTemplates } from "@sparkops/bff-modules/product";
import { CreateTemplateCommandSchema } from "@sparkops/contracts/product";
import { NextRequest } from "next/server";

export const GET = createBffHandler(async (ctx, _req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) {
    throw new Error("workspace required");
  }
  const items = await listTemplates(ctx, wsId);
  return { items };
});

export async function POST(req: NextRequest, context: { params: Promise<{ wsId: string }> }) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = CreateTemplateCommandSchema.parse(await req.json());
    const template = await createTemplate(ctx, params.wsId, body);
    return ok(template, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
}
