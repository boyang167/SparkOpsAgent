import type { BffContext } from "@sparkops/bff-core";
import type { ProductRequirementVM } from "@sparkops/contracts/product";

export async function listRequirements(ctx: BffContext, workspaceId: string, productId: string) {
  const data = await ctx.upstream.product.listRequirements(workspaceId, productId);
  return data.items;
}

export async function createRequirement(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  input: Record<string, unknown>,
) {
  return ctx.upstream.product.createRequirement(workspaceId, productId, input);
}

export async function approveRequirement(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  reqId: string,
) {
  return ctx.upstream.product.approveRequirement(workspaceId, productId, reqId);
}

export async function rejectRequirement(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  reqId: string,
  reason?: string,
) {
  return ctx.upstream.product.rejectRequirement(workspaceId, productId, reqId, reason);
}

export async function releaseRequirement(
  ctx: BffContext,
  workspaceId: string,
  productId: string,
  reqId: string,
) {
  return ctx.upstream.product.releaseRequirement(workspaceId, productId, reqId);
}

export type { ProductRequirementVM };
