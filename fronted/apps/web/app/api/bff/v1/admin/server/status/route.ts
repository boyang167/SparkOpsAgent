import { createBffHandler } from "@sparkops/bff-core";
import { getAdminServerStatus } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => getAdminServerStatus(ctx));
