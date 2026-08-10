import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createApiToken, listApiTokens } from "@sparkops/bff-modules/governance";
import { CreateAPITokenCommandSchema } from "@sparkops/contracts/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string }> };

export const GET = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId } = await params;
    const items = await listApiTokens(ctx, wsId);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId } = await params;
    const body = CreateAPITokenCommandSchema.parse(await req.json());
    const token = await createApiToken(ctx, wsId, body);
    return ok(token, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
};
