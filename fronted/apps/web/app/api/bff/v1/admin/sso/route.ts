import { createBffHandler } from "@sparkops/bff-core";
import {
  getSSOAdminConfig,
  updateSSOAdminConfig,
} from "@sparkops/bff-modules/identity";
import { UpdateSSOConfigCommandSchema } from "@sparkops/contracts/identity";

export const GET = createBffHandler(async (ctx) => getSSOAdminConfig(ctx));

export const PUT = createBffHandler(async (ctx, req) => {
  const input = UpdateSSOConfigCommandSchema.parse(await req.json());
  return updateSSOAdminConfig(ctx, input);
});
