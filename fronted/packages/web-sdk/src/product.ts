import { bffFetch } from "./client";
import type { ProductPlanVM, ProductVM } from "@sparkops/contracts/product";

export async function listProducts(workspaceId: string) {
  const data = await bffFetch<{ items: ProductVM[] }>(`/workspaces/${workspaceId}/products`, {
    workspaceId,
  });
  return data.items;
}

export async function listProductPlans(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: ProductPlanVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/plans`,
    { workspaceId },
  );
  return data.items;
}

export async function listRequirements(workspaceId: string, productId: string) {
  const data = await bffFetch<{ items: import("@sparkops/contracts/product").ProductRequirementVM[] }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements`,
    { workspaceId },
  );
  return data.items;
}

export function approveRequirement(workspaceId: string, productId: string, reqId: string) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/approve`,
    { method: "POST", workspaceId },
  );
}

export function rejectRequirement(
  workspaceId: string,
  productId: string,
  reqId: string,
  reason?: string,
) {
  return bffFetch<{ id: string; status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/reject`,
    { method: "POST", body: JSON.stringify({ reason }), workspaceId },
  );
}

export function releaseRequirement(workspaceId: string, productId: string, reqId: string) {
  return bffFetch<{ id: string; status: string; build_id?: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/release`,
    { method: "POST", workspaceId },
  );
}

export function crossReleaseRequirement(
  workspaceId: string,
  productId: string,
  reqId: string,
  input: { source_cluster: string; target_cluster: string },
) {
  return bffFetch<{ id: string; multicluster_status: string }>(
    `/workspaces/${workspaceId}/products/${productId}/requirements/${reqId}/cross-release`,
    { method: "POST", body: JSON.stringify(input), workspaceId },
  );
}
