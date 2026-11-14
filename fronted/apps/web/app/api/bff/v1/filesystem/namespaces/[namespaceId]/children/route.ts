import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listGovernedChildren } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { GovernedChildrenQuerySchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function GET(
  req: NextRequest,
  context: { params: Promise<{ namespaceId: string }> },
) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const { namespaceId } = await context.params;
    const search = req.nextUrl.searchParams;
    const rawLimit = search.get("limit");
    const query = GovernedChildrenQuerySchema.parse({
      parent_id: search.get("parent_id") || undefined,
      type: search.get("type") || undefined,
      q: search.get("q") || undefined,
      cursor: search.get("cursor") || undefined,
      limit: rawLimit == null ? undefined : Number(rawLimit),
    });
    const page = await listGovernedChildren(
      ctx,
      organizationId,
      ctx.workspaceId,
      namespaceId,
      query,
    );
    return ok(page, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
