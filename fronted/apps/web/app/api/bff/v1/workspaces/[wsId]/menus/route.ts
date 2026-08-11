import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createMenu } from "@sparkops/bff-modules/governance";
import { UpsertMenuCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string }> };

export const POST = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId } = await params;
    const body = UpsertMenuCommandSchema.parse(await req.json());
    const menu = await createMenu(ctx, wsId, body);
    return ok(menu, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
