import { createBffHandler } from "@sparkops/bff-core";
import { listTools } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => {
  return listTools(ctx);
});
