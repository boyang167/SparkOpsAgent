import { createBffHandler } from "@sparkops/bff-core";
import { testSSOProvider } from "@sparkops/bff-modules/identity";
import { TestSSOProviderCommandSchema } from "@sparkops/contracts/identity";

export const POST = createBffHandler(async (ctx, req) => {
  const input = TestSSOProviderCommandSchema.parse(await req.json());
  return testSSOProvider(ctx, input);
});
