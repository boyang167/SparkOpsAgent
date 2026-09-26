import { createBffHandler } from "@sparkops/bff-core";
import { listWorkspaceReleasePlans } from "@sparkops/bff-modules/product";

export const GET = createBffHandler(async (ctx, req, { params }) => {
  const wsId = params?.wsId ?? ctx.workspaceId;
  if (!wsId) throw new Error("workspace required");
  const url = new URL(req.url);
  return {
    items: await listWorkspaceReleasePlans(ctx, wsId, {
      status: url.searchParams.get("status") ?? undefined,
      from: url.searchParams.get("from") ?? undefined,
      to: url.searchParams.get("to") ?? undefined,
    }),
  };
});
