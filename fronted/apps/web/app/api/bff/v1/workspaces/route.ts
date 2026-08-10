import { createBffHandler } from "@sparkops/bff-core";
import { listWorkspaces } from "@sparkops/bff-modules/tenant";

export const GET = createBffHandler(async (ctx) => {
  const items = await listWorkspaces(ctx);
  return { items };
});
