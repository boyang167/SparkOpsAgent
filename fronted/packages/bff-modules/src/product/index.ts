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
  const data = await ctx.upstream.product.listTemplates(workspaceId);
  return data.items;
}

export async function getTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.getTemplate(workspaceId, templateId);
}

export async function createTemplate(
  ctx: BffContext,
  workspaceId: string,
  input: import("@sparkops/contracts/product").CreateTemplateCommand,
) {
  return ctx.upstream.product.createTemplate(workspaceId, input);
}

export async function updateTemplate(
  ctx: BffContext,
  workspaceId: string,
  templateId: string,
  input: import("@sparkops/contracts/product").UpdateTemplateCommand,
) {
  return ctx.upstream.product.updateTemplate(workspaceId, templateId, input);
}

export async function deleteTemplate(ctx: BffContext, workspaceId: string, templateId: string) {
  return ctx.upstream.product.deleteTemplate(workspaceId, templateId);
}

export async function listDockerfiles(ctx: BffContext, workspaceId: string) {
  return ctx.upstream.product.listDockerfiles(workspaceId);
}
