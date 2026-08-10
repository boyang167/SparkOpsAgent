import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createPlatformOrganization, listPlatformOrganizations } from "@sparkops/bff-modules/tenant";
import { CreatePlatformOrganizationCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

export const GET = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const items = await listPlatformOrganizations(ctx);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = CreatePlatformOrganizationCommandSchema.parse(await req.json());
    const org = await createPlatformOrganization(ctx, body);
    return ok(org, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
};
