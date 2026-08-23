import type { ProductPlanVM, ProductVM } from "@sparkops/contracts/product";
import type { UpstreamRequestOptions } from "./core";
import { upstreamFetch } from "./core";

export function createProductUpstream(options: UpstreamRequestOptions = {}) {
  return {
    async listProducts(workspaceId: string) {
      return upstreamFetch<{ items: ProductVM[] }>(
        `/api/v1/workspaces/${workspaceId}/products`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listProductPlans(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: ProductPlanVM[] }>(
        `/api/v1/products/${productId}/plans`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async listRequirements(workspaceId: string, productId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").ProductRequirementVM[] }>(
        `/api/v1/products/${productId}/requirements`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createRequirement(workspaceId: string, productId: string, input: Record<string, unknown>) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/requirements`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async approveRequirement(workspaceId: string, productId: string, reqId: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/requirements/${reqId}/approve`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async rejectRequirement(workspaceId: string, productId: string, reqId: string, reason?: string) {
      return upstreamFetch<{ id: string; status: string }>(
        `/api/v1/products/${productId}/requirements/${reqId}/reject`,
        { method: "POST", body: JSON.stringify({ reason }), options: { ...options, workspaceId } },
      );
    },

    async releaseRequirement(workspaceId: string, productId: string, reqId: string) {
      return upstreamFetch<{ id: string; status: string; build_id?: string }>(
        `/api/v1/products/${productId}/requirements/${reqId}/release`,
        { method: "POST", options: { ...options, workspaceId } },
      );
    },

    async listTemplates(workspaceId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").TemplateVM[] }>(
        `/api/v1/workspaces/${workspaceId}/templates`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async getTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<import("@sparkops/contracts/product").TemplateVM>(
        `/api/v1/workspaces/${workspaceId}/templates/${templateId}`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },

    async createTemplate(workspaceId: string, input: Record<string, unknown>) {
      return upstreamFetch<import("@sparkops/contracts/product").TemplateVM>(
        `/api/v1/workspaces/${workspaceId}/templates`,
        { method: "POST", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async updateTemplate(workspaceId: string, templateId: string, input: Record<string, unknown>) {
      return upstreamFetch<import("@sparkops/contracts/product").TemplateVM>(
        `/api/v1/workspaces/${workspaceId}/templates/${templateId}`,
        { method: "PATCH", body: JSON.stringify(input), options: { ...options, workspaceId } },
      );
    },

    async deleteTemplate(workspaceId: string, templateId: string) {
      return upstreamFetch<{ deleted: boolean }>(
        `/api/v1/workspaces/${workspaceId}/templates/${templateId}`,
        { method: "DELETE", options: { ...options, workspaceId } },
      );
    },

    async listDockerfiles(workspaceId: string) {
      return upstreamFetch<{ items: import("@sparkops/contracts/product").DockerfileVM[] }>(
        `/api/v1/workspaces/${workspaceId}/market/dockerfiles`,
        { method: "GET", options: { ...options, workspaceId } },
      );
    },
  };
}

export type ProductUpstream = ReturnType<typeof createProductUpstream>;
