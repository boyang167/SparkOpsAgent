import { createBffHandler } from "@sparkops/bff-core";
import { listFileAuditEvents } from "@sparkops/bff-modules/filesystem";
import { FileAuditQuerySchema } from "@sparkops/contracts/filesystem";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId;
  if (!wsId) {
    throw new Error("workspace id required");
  }
  const sp = req.nextUrl.searchParams;
  const query = FileAuditQuerySchema.parse({
    actor_id: sp.get("actor_id") ?? undefined,
    action: sp.get("action") ?? undefined,
    resource_id: sp.get("resource_id") ?? undefined,
    result: sp.get("result") ?? undefined,
    from: sp.get("from") ?? undefined,
    to: sp.get("to") ?? undefined,
    cursor: sp.get("cursor") ?? undefined,
    limit: sp.get("limit") ? Number.parseInt(sp.get("limit")!, 10) : undefined,
  });
  return listFileAuditEvents(ctx, wsId, query);
});
