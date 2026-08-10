import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { createOrganization, listOrganizations } from "@sparkops/bff-modules/tenant";
import { CreateOrganizationCommandSchema } from "@sparkops/contracts/tenant";
import { NextRequest } from "next/server";

export const GET = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const items = await listOrganizations(ctx);
    return ok({ items }, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
};

export const POST = async (req: NextRequest) => {
  const traceId = createTraceId(req);
  try {
    const ctx = await createBffContext(req);
    const body = CreateOrganizationCommandSchema.parse(await req.json());
    const org = await createOrganization(ctx, body);
    return ok(org, traceId, 201);
  } catch (err) {
    return fail(err, traceId);
  }
};
