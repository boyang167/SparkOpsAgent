import { createBffHandler } from "@sparkops/bff-core";
import { getSysConfig, updateSysConfig } from "@sparkops/bff-modules/platform";

export const GET = createBffHandler(async (ctx) => getSysConfig(ctx));

export const PATCH = createBffHandler(async (ctx, req) => {
  const body = (await req.json()) as Record<string, unknown>;
  return updateSysConfig(ctx, body);
});
