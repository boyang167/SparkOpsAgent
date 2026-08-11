import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { deleteTemplate, getTemplate, updateTemplate } from "@sparkops/bff-modules/product";
import { UpdateTemplateCommandSchema } from "@sparkops/contracts/product";
import { NextRequest } from "next/server";

export async function GET(
  _req: NextRequest,
  context: { params: Promise<{ wsId: string; templateId: string }> },
) {
  const traceId = createTraceId(_req);
  try {
    const ctx = await createBffContext(_req);
    const params = await context.params;
    const template = await getTemplate(ctx, params.wsId, params.templateId);
    return ok(template, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function PATCH(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; templateId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const body = UpdateTemplateCommandSchema.parse(await req.json());
    const template = await updateTemplate(ctx, params.wsId, params.templateId, body);
    return ok(template, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}

export async function DELETE(
  req: NextRequest,
  context: { params: Promise<{ wsId: string; templateId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const params = await context.params;
    const result = await deleteTemplate(ctx, params.wsId, params.templateId);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
