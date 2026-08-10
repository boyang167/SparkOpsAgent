import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { revokeApiToken } from "@sparkops/bff-modules/governance";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ wsId: string; tokenId: string }> };

export const DELETE = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { wsId, tokenId } = await params;
    await revokeApiToken(ctx, wsId, tokenId);
    return ok({ revoked: true }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
