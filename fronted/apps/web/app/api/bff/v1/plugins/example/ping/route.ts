import { createBffHandler } from "@sparkops/bff-core";
import { getExamplePing } from "@sparkops/bff-modules/plugins";

export const GET = createBffHandler(async (ctx) => getExamplePing(ctx));
