import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { transferPlatformOrgOwner } from "@sparkops/bff-modules/tenant";
import { TransferOrgOwnerCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

type RouteParams = { params: Promise<{ orgId: string }> };

export const POST = async (req: NextRequest, { params }: RouteParams) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const { orgId } = await params;
    const body = TransferOrgOwnerCommandSchema.parse(await req.json());
    const result = await transferPlatformOrgOwner(ctx, orgId, body);
    return ok(result, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};
