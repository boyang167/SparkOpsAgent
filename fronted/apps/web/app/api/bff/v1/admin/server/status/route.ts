import { createBffHandler } from "@sparkops/bff-core";
import { getServerStatus } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => getServerStatus(ctx));
