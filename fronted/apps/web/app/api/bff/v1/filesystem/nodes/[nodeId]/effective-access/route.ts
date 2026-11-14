import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { getEffectiveAccess } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ nodeId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const { nodeId } = await context.params;
    const access = await getEffectiveAccess(ctx, organizationId, ctx.workspaceId, nodeId);
    return ok(access, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
