import { createBffHandler } from "@sparkops/bff-core";
import { listModels } from "@sparkops/bff-modules/integration";

export const GET = createBffHandler(async (ctx) => listModels(ctx));
