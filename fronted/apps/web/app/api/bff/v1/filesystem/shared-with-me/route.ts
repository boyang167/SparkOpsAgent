import { createBffContext, createTraceId, fail, ok } from "@sparkops/bff-core";
import { listSharedWithMe } from "@sparkops/bff-modules/filesystem";
import { BffError } from "@sparkops/contracts";
import { SharedWithMeQuerySchema } from "@sparkops/contracts/filesystem";
import { NextRequest } from "next/server";

export async function GET(req: NextRequest) {
  const traceId = createTraceId(req);
  try {
    const organizationId = req.headers.get("x-organization-id")?.trim();
    if (!organizationId) {
      throw new BffError("X-Organization-ID header required", "TENANT_ORG_REQUIRED", 400, traceId);
    }
    const ctx = await createBffContext(req);
    const search = req.nextUrl.searchParams;
    const rawLimit = search.get("limit");
    const query = SharedWithMeQuerySchema.parse({
      q: search.get("q") || undefined,
      cursor: search.get("cursor") || undefined,
      limit: rawLimit == null ? undefined : Number(rawLimit),
    });
    const page = await listSharedWithMe(ctx, organizationId, ctx.workspaceId, query);
    return ok(page, traceId);
  } catch (err) {
    return fail(err, traceId);
  }
}
