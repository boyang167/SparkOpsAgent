import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listOrganizationFileAuditEvents } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { FileAuditQuerySchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const query = FileAuditQuerySchema.parse(
      Object.fromEntries(req.nextUrl.searchParams.entries()),
    );
    return ok(
      await listOrganizationFileAuditEvents(ctx, organizationId, ctx.workspaceId, query),
      traceId,
    );
  } catch (error) {
    return fail(error, traceId);
  }
}
