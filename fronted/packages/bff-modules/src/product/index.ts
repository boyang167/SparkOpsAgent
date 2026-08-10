import type { BffContext } from "@sparkops/bff-core";

export async function listProducts(ctx: BffContext, workspaceId: string) {
  const data = await ctx.upstream.product.listProducts(workspaceId);
  return data.items;
}

export async function listProductPlans(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listProductPlans(workspaceId, productId);
  return data.items;
}

export {
  listRequirements,
  createRequirement,
  approveRequirement,
  rejectRequirement,
  releaseRequirement,
  crossReleaseRequirement,
} from "./requirements";

export async function listTemplates(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.product.listTemplates(workspaceId);
}

export async function listDockerfiles(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.product.listDockerfiles(workspaceId);
}
