import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { updateGeneratedTemplate } from "@sparkops/bff-modules/workspace";
import { UpdateGeneratedTemplateCommandSchema } from "@sparkops/contracts/workspace";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; projectId: string; kind: string }> };

export const PUT = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, projectId, kind } = await params;
    const body = UpdateGeneratedTemplateCommandSchema.parse(await req.json());
    const data = await updateGeneratedTemplate(ctx, wsId, projectId, kind, body);
    return ok(data, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
