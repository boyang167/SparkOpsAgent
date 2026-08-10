import { createBffHandler } from "@sparkops/bff-core";
import { listNotifications } from "@sparkops/bff-modules/notification";

export const GET = createBffHandler(async (ctx) => {
  if (!ctx.workspaceId) {
    throw new Error("workspace required");
  }
  return listNotifications(ctx, ctx.workspaceId);
});
