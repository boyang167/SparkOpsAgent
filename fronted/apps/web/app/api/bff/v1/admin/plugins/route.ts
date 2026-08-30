import { createBffHandler } from "@sparkops/bff-core";
import { listPlugins } from "@sparkops/bff-modules/plugins";

export const GET = createBffHandler(async (ctx) => ({
  items: await listPlugins(ctx),
}));
