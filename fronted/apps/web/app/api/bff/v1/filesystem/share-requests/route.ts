import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listOrganizationShareRequests } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { OrganizationShareRequestQuerySchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const query = OrganizationShareRequestQuerySchema.parse(
      Object.fromEntries(req.nextUrl.searchParams.entries()),
    );
    return ok(
      await listOrganizationShareRequests(ctx, organizationId, ctx.workspaceId, query),
      traceId,
    );
  } catch (error) {
    return fail(error, traceId);
  }
}
