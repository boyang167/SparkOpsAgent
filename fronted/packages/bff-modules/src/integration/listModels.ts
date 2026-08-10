import type { BffContext } from "@sparkops/bff-core";
import type { ModelVM } from "@sparkops/contracts/integration";

export async function listModels(ctx: BffContext): Promise<ModelVM[]> {
  const data = await ctx.upstream.integration.listModels();
  return data.items;
}
